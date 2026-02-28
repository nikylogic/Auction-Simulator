// Package config handles all the settings for the simulator.
// It reads what the user wants (like how many bidders, how many auctions, etc.)
// from command-line flags or environment variables,
// and also sets limits on how much CPU and memory the program can use.
package config

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"time"
)

// Config holds every setting the simulator needs to run.
// You can think of this as the "control panel" — all the knobs you can turn.
type Config struct {
	NumBidders     int           // how many bidders compete in each auction (default: 100)
	NumAuctions    int           // how many auctions run at the same time (default: 40)
	NumAttributes  int           // how many properties each auction object has (default: 20)
	AuctionTimeout time.Duration // how long each auction stays open before closing (default: 200ms)
	MaxCPU         int           // max number of CPU threads the program can use
	MaxMemoryMB    int64         // max memory in megabytes the program should try to stay under
}

// Parse reads the settings from command-line flags (like --bidders 100)
// or from environment variables (like AUCTION_BIDDERS=100).
// If both are set, the command-line flag wins.
func Parse() *Config {
	cfg := &Config{}

	// Define each flag with a name, a default value, and a short description.
	// The envInt helper checks if an environment variable is set first —
	// if it is, that becomes the default instead of the hardcoded one.
	flag.IntVar(&cfg.NumBidders, "bidders", envInt("AUCTION_BIDDERS", 100), "Number of bidders per auction")
	flag.IntVar(&cfg.NumAuctions, "auctions", envInt("AUCTION_COUNT", 40), "Number of auctions to run concurrently")
	flag.IntVar(&cfg.NumAttributes, "attributes", envInt("AUCTION_ATTRIBUTES", 20), "Number of attributes per auctioned object")

	timeoutMs := flag.Int("timeout", envInt("AUCTION_TIMEOUT_MS", 200), "Auction timeout in milliseconds")
	flag.IntVar(&cfg.MaxCPU, "vcpu", envInt("AUCTION_VCPU", runtime.NumCPU()), "Max vCPU (OS threads) to use")

	var maxMem int
	flag.IntVar(&maxMem, "memory-mb", envInt("AUCTION_MEMORY_MB", 512), "Max memory in MB (soft limit)")

	// Actually read the flags from the command line
	flag.Parse()

	// Convert the timeout from milliseconds to Go's time.Duration type
	cfg.AuctionTimeout = time.Duration(*timeoutMs) * time.Millisecond
	cfg.MaxMemoryMB = int64(maxMem)

	return cfg
}

// Apply sets the resource limits on the Go runtime.
// This is how we "standardize" resources —
// even if your machine has 16 cores and 32GB RAM,
// we can make the program behave as if it only has 2 cores and 256MB.
func (c *Config) Apply() {
	// Tell Go's scheduler: "you can only use this many OS threads at once."
	// If we set this to 2, only 2 goroutines can truly run in parallel,
	// even if we have thousands of goroutines alive.
	prev := runtime.GOMAXPROCS(c.MaxCPU)
	fmt.Printf("[Resource] GOMAXPROCS: %d → %d\n", prev, c.MaxCPU)

	// Tell the garbage collector: "try to keep memory usage below this limit."
	// It's a soft limit — the program won't crash if it goes over,
	// but the GC will work harder to free up memory and stay under.
	memBytes := c.MaxMemoryMB * 1024 * 1024
	debug.SetMemoryLimit(memBytes)
	fmt.Printf("[Resource] Memory limit: %d MB\n", c.MaxMemoryMB)
}

// Print shows the current configuration in a nice box on the terminal.
// This runs right at the start so you can see what settings are active.
func (c *Config) Print() {
	fmt.Println("╔══════════════════════════════════════════╗")
	fmt.Println("║        AUCTION SIMULATOR CONFIG          ║")
	fmt.Println("╠══════════════════════════════════════════╣")
	fmt.Printf("║  Bidders:        %-24d║\n", c.NumBidders)
	fmt.Printf("║  Auctions:       %-24d║\n", c.NumAuctions)
	fmt.Printf("║  Attributes:     %-24d║\n", c.NumAttributes)
	fmt.Printf("║  Timeout:        %-24s║\n", c.AuctionTimeout)
	fmt.Printf("║  vCPU (threads): %-24d║\n", c.MaxCPU)
	fmt.Printf("║  Memory limit:   %-21s MB ║\n", fmt.Sprintf("%d", c.MaxMemoryMB))
	fmt.Println("╚══════════════════════════════════════════╝")
}

// envInt is a small helper that reads a number from an environment variable.
// If the variable doesn't exist or isn't a valid number, it returns the fallback value.
// Example: envInt("AUCTION_BIDDERS", 100) → returns 100 if AUCTION_BIDDERS isn't set.
func envInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
