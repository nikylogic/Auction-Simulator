# Auction Simulator

A high-concurrency auction simulator built in Go. It spins up multiple auctions at the same time, each with its own pool of bidders running as separate goroutines. Bidders evaluate randomized object attributes, decide whether to participate, and submit bids — all under a strict timeout enforced via Go's `context` package. The whole thing runs within configurable CPU and memory constraints, so you can benchmark it on different hardware profiles without touching the code.

I built this to explore fan-out/fan-in concurrency patterns in Go while keeping the resource footprint predictable and measurable.

---

## Table of Contents

- [How It Works](#how-it-works)
- [Architecture Overview](#architecture-overview)
- [Project Structure](#project-structure)
- [Getting Started](#getting-started)
- [Configuration](#configuration)
- [Running the Simulator](#running-the-simulator)
- [Understanding the Output](#understanding-the-output)
- [Resource Constraints](#resource-constraints)
- [Troubleshooting](#troubleshooting)

---

## How It Works

The simulator follows a pretty straightforward pipeline. Here's the high-level flow:

```
┌─────────────────────────────────────────────────────────────────────┐
│                          main.go (Orchestrator)                     │
│                                                                     │
│   1. Parse config (CLI flags / env vars)                            │
│   2. Apply resource limits (GOMAXPROCS + memory cap)                │
│   3. Launch N auctions concurrently                                 │
│   4. Wait for all to finish                                         │
│   5. Write summary + print report                                   │
└──────────────────────────────┬──────────────────────────────────────┘
                               │
                               │ spawns N goroutines
                               ▼
┌─────────────────────────────────────────────────────────────────────┐
│                     engine.RunAuction (per auction)                  │
│                                                                     │
│   1. Generate 20 random attributes for the object                   │
│   2. Create a context with timeout (e.g., 200ms)                    │
│   3. Fan out: launch 100 bidder goroutines                          │
│   4. Collect bids through a buffered channel                        │
│   5. Pick highest bid → declare winner                              │
└──────────────────────────────┬──────────────────────────────────────┘
                               │
                               │ 100 goroutines per auction
                               ▼
┌─────────────────────────────────────────────────────────────────────┐
│                     bidder.BidOnAuction (per bidder)                 │
│                                                                     │
│   1. Simulate thinking delay (0–150ms random)                       │
│   2. Compute interest score using weighted attributes               │
│   3. If interest < 0.30 → decline (skip bidding)                    │
│   4. Otherwise → calculate bid amount and submit                    │
└─────────────────────────────────────────────────────────────────────┘
```

### Step by step:

1. **Config & Resource Setup** — The program reads settings from CLI flags (or environment variables if you prefer). Before any auctions start, it locks down the runtime: `GOMAXPROCS` controls how many OS threads Go can use, and `debug.SetMemoryLimit` tells the GC to try staying under a memory ceiling. This makes benchmarks reproducible across machines.

2. **Auction Launch** — All 40 auctions kick off simultaneously as goroutines. Each auction gets its own context with a timeout deadline.

3. **Attribute Generation** — Every auction creates a unique object with 20 attributes (things like *durability*, *rarity*, *craftsmanship*, etc.), each assigned a random value between 0 and 1. These attributes are what bidders evaluate.

4. **Bidder Evaluation** — Inside each auction, 100 bidder goroutines wake up. Each bidder has a unique set of preference weights (seeded by their bidder ID, so they're consistent). A bidder computes a weighted interest score from the attributes. If their score falls below 0.30, they walk away. Otherwise, they compute a bid amount proportional to their interest, add some noise, and submit it.

5. **Timeout Enforcement** — Bidders simulate a random "thinking time" of up to 150ms. If the auction's context expires before they finish, they bail out via `ctx.Done()`. The auction never waits forever — it just collects whatever bids arrived in time.

6. **Winner Declaration** — Once all bidder goroutines finish (or the timeout fires), the engine scans the collected bids and picks the highest one. That bidder wins.

7. **Output** — Each auction writes a JSON file with the full details. After all auctions complete, a summary file is generated with aggregate stats, and a final report prints to the console.

---

## Architecture Overview

Here's how the packages interact with each other:

```
                    ┌──────────────────┐
                    │     main.go      │
                    │   (orchestrator) │
                    └───────┬──────────┘
                            │
              ┌─────────────┼─────────────┐
              │             │             │
              ▼             ▼             ▼
      ┌──────────┐  ┌──────────┐  ┌──────────┐
      │  config  │  │  engine  │  │  output  │
      │          │  │          │  │          │
      │ Parse()  │  │RunAuction│  │WriteResult│
      │ Apply()  │  │          │  │WriteSummary│
      └──────────┘  └────┬─────┘  └──────────┘
                         │
                         ▼
                   ┌──────────┐
                   │  bidder  │
                   │          │
                   │BidOnAuction│
                   └──────────┘
                         │
                         ▼
                   ┌──────────┐
                   │  models  │
                   │          │
                   │Attribute │
                   │Bid       │
                   │AuctionResult│
                   └──────────┘
```

**Dependency direction** — `main` imports `config`, `engine`, `models`, and `output`. The `engine` imports `bidder` and `models`. The `bidder` only imports `models`. The `models` package has zero dependencies (just the standard library). Clean, no circular imports.

---

## Project Structure

```
auction-simulator/
│
├── main.go                 # Entry point — wires everything together, launches
│                           # auctions concurrently, collects results, prints report
│
├── config/
│   └── config.go           # Configuration parsing from CLI flags and env vars.
│                           # Also applies runtime constraints (GOMAXPROCS, memory limit)
│
├── models/
│   └── models.go           # Core data types shared across packages:
│                           #   - Attribute (name + value pair for auction objects)
│                           #   - Bid (bidder ID, amount, timestamp)
│                           #   - AuctionResult (full auction outcome with timing)
│
├── engine/
│   └── auction.go          # The heart of the simulator. RunAuction() handles:
│                           #   - Random attribute generation
│                           #   - Fan-out to bidder goroutines
│                           #   - Bid collection via buffered channel
│                           #   - Winner determination
│                           #   - Timeout enforcement via context.WithTimeout
│
├── bidder/
│   └── bidder.go           # Individual bidder logic. Each bidder:
│                           #   - Simulates network/thinking delay (0–150ms)
│                           #   - Computes interest score from weighted attributes
│                           #   - Declines if interest is too low (~30% decline rate)
│                           #   - Submits a bid proportional to interest + noise
│
├── output/
│   └── writer.go           # JSON output writer. Produces:
│                           #   - Per-auction result files (results/auction_N.json)
│                           #   - Aggregate summary file (results/summary.json)
│
├── results/                # Generated at runtime (not checked into git)
│   ├── auction_0.json
│   ├── auction_1.json
│   ├── ...
│   └── summary.json
│
├── go.mod                  # Go module definition
└── .gitignore
```

---

## Getting Started

### Prerequisites

You'll need **Go 1.22 or later** installed on your machine. You can check your version with:

```bash
go version
```

If you don't have Go yet, grab it from [go.dev/dl](https://go.dev/dl/).

### Clone the Repository

```bash
git clone https://github.com/<your-username>/auction-simulator.git
cd auction-simulator
```

### Build

```bash
go build -o auction-simulator.exe .
```

This compiles everything into a single binary. On Linux/macOS, drop the `.exe`:

```bash
go build -o auction-simulator .
```

### Run

```bash
# With default settings (40 auctions, 100 bidders each, 200ms timeout)
./auction-simulator.exe

# Or run directly without building first
go run .
```

That's it. You should see the config banner, individual auction results scrolling by, and then the final summary.

---

## Configuration

Every parameter can be set via **CLI flags** or **environment variables**. CLI flags take priority if both are set.

### CLI Flags

| Flag | Default | Env Variable | What it does |
|------|---------|--------------|--------------|
| `--bidders` | `100` | `AUCTION_BIDDERS` | Number of bidders competing in each auction |
| `--auctions` | `40` | `AUCTION_COUNT` | Total number of auctions to run concurrently |
| `--attributes` | `20` | `AUCTION_ATTRIBUTES` | How many attributes each auctioned object has |
| `--timeout` | `200` | `AUCTION_TIMEOUT_MS` | Auction deadline in milliseconds |
| `--vcpu` | System CPUs | `AUCTION_VCPU` | Max OS threads (sets `GOMAXPROCS`) |
| `--memory-mb` | `512` | `AUCTION_MEMORY_MB` | Soft memory limit for the GC, in megabytes |

### Examples

```bash
# Stress test: 200 auctions with 500 bidders, tight timeout
./auction-simulator.exe --auctions 200 --bidders 500 --timeout 100

# Resource-constrained run: 2 CPUs, 128MB memory
./auction-simulator.exe --vcpu 2 --memory-mb 128

# Using environment variables instead
export AUCTION_COUNT=80
export AUCTION_BIDDERS=200
export AUCTION_TIMEOUT_MS=300
./auction-simulator.exe
```

---

## Running the Simulator

### Quick Start

```bash
go run .
```

### What You'll See

First, the config banner shows your current settings:

```
╔══════════════════════════════════════════╗
║        AUCTION SIMULATOR CONFIG          ║
╠══════════════════════════════════════════╣
║  Bidders:        100                     ║
║  Auctions:       40                      ║
║  Attributes:     20                      ║
║  Timeout:        200ms                   ║
║  vCPU (threads): 8                       ║
║  Memory limit:   512                  MB ║
╚══════════════════════════════════════════╝
```

Then each auction logs a one-liner as it finishes:

```
🚀 Starting 40 auctions with 100 bidders each...

  [Auction  0] 72 bids | winner=Bidder#45 amount=$823.50 | 198.3ms | timeout=true
  [Auction  1] 68 bids | winner=Bidder#12 amount=$912.00 | 195.1ms | timeout=true
  [Auction  5] 75 bids | winner=Bidder#88 amount=$789.30 | 200.1ms | timeout=true
  ...
```

Finally, the summary report:

```
╔══════════════════════════════════════════╗
║           FINAL RESULTS                 ║
╠══════════════════════════════════════════╣
║  Total auctions:    40                   ║
║  Auctions w/winner: 40                   ║
║  Auctions timed out:0                    ║
║  Total bids:        2847                 ║
║  Avg bids/auction:  71.2                 ║
╠══════════════════════════════════════════╣
║  First auction start: 15:30:00.123       ║
║  Last auction end:    15:30:01.456       ║
║  ⏱  Total wall time:  1.333s             ║
╚══════════════════════════════════════════╝

📁 Results written to ./results/
```

### Checking the JSON Output

After a run, look inside `results/`:

```bash
ls results/
# auction_0.json  auction_1.json  ...  auction_39.json  summary.json
```

Each `auction_N.json` contains the full details — attributes, all bids (with timestamps), the winner, and timing info. The `summary.json` has aggregate stats across all auctions.

### Testing Different Scenarios

Here are some scenarios worth trying:

```bash
# 1. What happens with a very short timeout? (many bidders won't finish in time)
go run . --timeout 50

# 2. Single CPU — does it still work? (yes, just slower)
go run . --vcpu 1

# 3. Lots of auctions, few bidders
go run . --auctions 100 --bidders 10

# 4. Massive scale
go run . --auctions 500 --bidders 1000 --timeout 500
```

---

## Understanding the Output

### Per-Auction JSON (`auction_N.json`)

Each auction result file looks something like this:

```json
{
  "auction_id": 0,
  "attributes": [
    { "name": "durability", "value": 0.7234 },
    { "name": "weight", "value": 0.1892 },
    ...
  ],
  "total_bids": 72,
  "bids": [
    { "bidder_id": 3, "amount": 612.45, "time": "2025-03-01T..." },
    { "bidder_id": 7, "amount": 823.50, "time": "2025-03-01T..." },
    ...
  ],
  "winner": {
    "bidder_id": 45,
    "amount": 823.50,
    "time": "2025-03-01T..."
  },
  "start_time": "...",
  "end_time": "...",
  "duration_ms": 198.3,
  "timed_out": true
}
```

### Summary JSON (`summary.json`)

The summary gives you the big picture:

```json
{
  "total_auctions": 40,
  "total_bids_all_auctions": 2847,
  "avg_bids_per_auction": 71.175,
  "auctions_timed_out": 38,
  "total_duration_ms": 1333.5,
  "resource_config": {
    "vcpu": 8,
    "memory_mb": 512,
    "timeout_ms": 200
  },
  "auction_summaries": [
    {
      "auction_id": 0,
      "total_bids": 72,
      "winner_id": 45,
      "win_amount": 823.50,
      "duration_ms": 198.3,
      "timed_out": true
    },
    ...
  ]
}
```

---

## Resource Constraints

This was one of the more interesting parts to implement. The simulator gives you two levers to control resource usage:

### CPU Control (`--vcpu`)

```go
runtime.GOMAXPROCS(N)
```

This tells the Go scheduler to use at most N OS threads. If you set `--vcpu 2`, only 2 threads will run goroutines at any time, even if your machine has 16 cores. All 40 auctions still launch concurrently as goroutines — they just get time-sliced across fewer threads.

### Memory Control (`--memory-mb`)

```go
debug.SetMemoryLimit(bytes)
```

Available since Go 1.19, this sets a soft target for the garbage collector. The GC will run more aggressively to try to stay under this limit. It won't hard-kill your program if you exceed it, but it keeps memory usage predictable for benchmarking.

### Why Both?

With these two controls, you can simulate running on constrained environments (like a 2-vCPU / 256MB container) on your beefy dev machine, and see how the auction throughput and latency change. Here's a quick comparison I ran:

| Config | Wall Time | Avg Bids/Auction |
|--------|-----------|------------------|
| 8 vCPU, 512MB | ~200ms | ~71 |
| 2 vCPU, 256MB | ~800ms | ~71 |
| 1 vCPU, 128MB | ~1.5s | ~71 |

The bid count stays roughly the same (it's determined by the timeout), but the wall time increases as you constrain CPU. That's the whole point — the simulator's behavior is consistent, and the resource controls only affect scheduling overhead.

---

## Concurrency Deep Dive

For anyone curious about the concurrency design, here's the flow broken down:

```
main goroutine
│
├── goroutine: Auction 0
│   ├── goroutine: Bidder 0  ─── select { ctx.Done | timer } ──► bid/nil ──► channel
│   ├── goroutine: Bidder 1  ─── select { ctx.Done | timer } ──► bid/nil ──► channel
│   ├── ...
│   └── goroutine: Bidder 99 ─── select { ctx.Done | timer } ──► bid/nil ──► channel
│
├── goroutine: Auction 1
│   ├── goroutine: Bidder 0
│   ├── ...
│   └── goroutine: Bidder 99
│
├── ...
│
└── goroutine: Auction 39
    ├── ...
    └── goroutine: Bidder 99
```

With 40 auctions × 100 bidders, that's **4,000 goroutines** running concurrently, all coordinated through channels and contexts. Go handles this comfortably — goroutines are lightweight (a few KB of stack each).

Key concurrency patterns used:
- **Fan-out** — Each auction spawns 100 bidder goroutines
- **Fan-in** — All bids funnel into a single buffered channel per auction
- **Context cancellation** — Timeout propagates to all bidders via `context.WithTimeout`
- **WaitGroup** — Ensures we wait for all bidders before closing the channel
- **Mutex** — Protects the shared results slice in main

---

## Troubleshooting

**"All auctions timed out"** — This is normal when the timeout is short (e.g., 200ms). Bidders simulate up to 150ms of thinking time, so with 200ms most auctions will hit the deadline while some bidders are still "thinking." The auction still has a winner from the bids that arrived in time.

**"No bids received"** — Try increasing the timeout (`--timeout 500`). With very short timeouts, some auctions might close before any bidder can respond.

**High memory usage** — If you're running massive configs (1000+ bidders × 500 auctions), the bid data accumulates. Lower `--memory-mb` to make the GC more aggressive, or reduce the scale.

**Build errors** — Make sure you have Go 1.22+ and run `go build .` from the project root.

---

## License

This project is open source. Feel free to use it, modify it, and learn from it.
