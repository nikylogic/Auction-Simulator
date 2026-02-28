// This is the entry point of the auction simulator.
// Think of main.go as the "director" — it doesn't run any auctions itself,
// but it tells everyone else what to do:
//  1. Read the settings (how many auctions, bidders, timeout, etc.)
//  2. Set the resource limits (CPU and memory caps)
//  3. Launch all 40 auctions at the same time
//  4. Wait for them all to finish
//  5. Save the results and print a final report
package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/auction-simulator/config"
	"github.com/auction-simulator/engine"
	"github.com/auction-simulator/models"
	"github.com/auction-simulator/output"
)

func main() {
	// ─── STEP 1: Read the settings ─────────────────────────────────
	// Parse all the configuration from command-line flags or environment variables.
	// Then print a nice banner showing what settings are active.
	cfg := config.Parse()
	cfg.Print()

	// ─── STEP 2: Lock down the resources ───────────────────────────
	// This is where resource standardization happens.
	// GOMAXPROCS limits CPU threads, SetMemoryLimit caps memory.
	// After this line, the program runs within the specified constraints.
	cfg.Apply()

	// This is where the output files will go
	resultsDir := "results"

	// ─── STEP 3: Launch ALL auctions concurrently ──────────────────
	fmt.Printf("\n🚀 Starting %d auctions with %d bidders each...\n\n", cfg.NumAuctions, cfg.NumBidders)

	// Record the start time — this is the "start of the first auction"
	// that we need to measure for the time measurement requirement.
	overallStart := time.Now()

	// Create a slice to hold all 40 results, a mutex to protect it
	// (since multiple goroutines will write to it), and a WaitGroup
	// to know when all auctions are done.
	results := make([]*models.AuctionResult, cfg.NumAuctions)
	var mu sync.Mutex
	var wg sync.WaitGroup

	ctx := context.Background()

	// Launch all 40 auctions as separate goroutines — they ALL start at the same time.
	// This is real concurrency: 40 auctions × 100 bidders = 4,000 goroutines running.
	// GOMAXPROCS controls how many of these actually run in parallel on OS threads.
	for i := 0; i < cfg.NumAuctions; i++ {
		wg.Add(1) // "one more auction is starting"
		go func(auctionID int) {
			defer wg.Done() // "this auction is done"

			// Run the auction — this handles everything:
			// attribute generation, bidder fan-out, bid collection, winner selection
			result := engine.RunAuction(ctx, auctionID, cfg.NumBidders, cfg.NumAttributes, cfg.AuctionTimeout)

			// Write the result to a JSON file (e.g., results/auction_0.json)
			if err := output.WriteResult(resultsDir, result); err != nil {
				fmt.Printf("  ⚠ Error writing auction %d result: %v\n", auctionID, err)
			}

			// Store the result in our shared slice (protected by a mutex
			// because multiple goroutines might try to write at the same time)
			mu.Lock()
			results[auctionID] = result
			mu.Unlock()
		}(i)
	}

	// Wait here until ALL 40 auctions have finished
	wg.Wait()

	// Record the end time — this is "completion of the last auction"
	overallEnd := time.Now()

	// ─── STEP 4: Write the summary file ────────────────────────────
	// This creates "results/summary.json" with aggregate stats from all auctions.
	timeoutMs := int(cfg.AuctionTimeout.Milliseconds())
	if err := output.WriteSummary(resultsDir, results, overallStart, overallEnd, cfg.MaxCPU, cfg.MaxMemoryMB, timeoutMs); err != nil {
		fmt.Printf("⚠ Error writing summary: %v\n", err)
	}

	// ─── STEP 5: Print the final report ────────────────────────────
	// Calculate some totals for the console output
	totalDuration := overallEnd.Sub(overallStart)
	totalBids := 0
	auctionsWithWinner := 0
	timedOutCount := 0

	for _, r := range results {
		if r != nil {
			totalBids += r.TotalBids
			if r.Winner != nil {
				auctionsWithWinner++
			}
			if r.TimedOut {
				timedOutCount++
			}
		}
	}

	// Print a nice formatted box with the final numbers
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════╗")
	fmt.Println("║           FINAL RESULTS                 ║")
	fmt.Println("╠══════════════════════════════════════════╣")
	fmt.Printf("║  Total auctions:    %-21d║\n", cfg.NumAuctions)
	fmt.Printf("║  Auctions w/winner: %-21d║\n", auctionsWithWinner)
	fmt.Printf("║  Auctions timed out:%-21d║\n", timedOutCount)
	fmt.Printf("║  Total bids:        %-21d║\n", totalBids)
	fmt.Printf("║  Avg bids/auction:  %-21.1f║\n", float64(totalBids)/float64(cfg.NumAuctions))
	fmt.Println("╠══════════════════════════════════════════╣")
	fmt.Printf("║  First auction start: %s  ║\n", overallStart.Format("15:04:05.000"))
	fmt.Printf("║  Last auction end:    %s  ║\n", overallEnd.Format("15:04:05.000"))
	fmt.Printf("║  ⏱  Total wall time:  %-18s ║\n", totalDuration.Round(time.Millisecond))
	fmt.Println("╚══════════════════════════════════════════╝")
	fmt.Printf("\n📁 Results written to ./%s/\n", resultsDir)
}
