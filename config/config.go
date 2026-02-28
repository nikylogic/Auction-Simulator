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

// Config holds all tunable parameters for the auction simulator.
type Config struct {
	NumBidders     int
	NumAuctions    int
	NumAttributes  int
	AuctionTimeout time.Duration
	MaxCPU         int
	MaxMemoryMB    int64
}

// Parse reads configuration from CLI flags and environment variables.
// CLI flags take precedence over environment variables.
func Parse() *Config {
	cfg := &Config{}

	// Define CLI flags with defaults
	flag.IntVar(&cfg.NumBidders, "bidders", envInt("AUCTION_BIDDERS", 100), "Number of bidders per auction")
	flag.IntVar(&cfg.NumAuctions, "auctions", envInt("AUCTION_COUNT", 40), "Number of auctions to run concurrently")
	flag.IntVar(&cfg.NumAttributes, "attributes", envInt("AUCTION_ATTRIBUTES", 20), "Number of attributes per auctioned object")

	timeoutMs := flag.Int("timeout", envInt("AUCTION_TIMEOUT_MS", 200), "Auction timeout in milliseconds")
	flag.IntVar(&cfg.MaxCPU, "vcpu", envInt("AUCTION_VCPU", runtime.NumCPU()), "Max vCPU (OS threads) to use")

	var maxMem int
	flag.IntVar(&maxMem, "memory-mb", envInt("AUCTION_MEMORY_MB", 512), "Max memory in MB (soft limit)")

	flag.Parse()

	cfg.AuctionTimeout = time.Duration(*timeoutMs) * time.Millisecond
	cfg.MaxMemoryMB = int64(maxMem)

	return cfg
}

// Apply sets the resource constraints on the Go runtime.
func (c *Config) Apply() {
	// Cap goroutine scheduling to N OS threads
	prev := runtime.GOMAXPROCS(c.MaxCPU)
	fmt.Printf("[Resource] GOMAXPROCS: %d → %d\n", prev, c.MaxCPU)

	// Set soft memory limit (Go 1.19+)
	memBytes := c.MaxMemoryMB * 1024 * 1024
	debug.SetMemoryLimit(memBytes)
	fmt.Printf("[Resource] Memory limit: %d MB\n", c.MaxMemoryMB)
}

// Print outputs the current configuration to stdout.
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

// envInt reads an integer from an environment variable, returning fallback if unset.
func envInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
