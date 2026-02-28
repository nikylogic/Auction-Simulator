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
	// 1. Parse configuration
	cfg := config.Parse()
	cfg.Print()

	// 2. Apply resource constraints (GOMAXPROCS + memory limit)
	cfg.Apply()

	// 3. Prepare results directory
	resultsDir := "results"

	// 4. Run all auctions concurrently with a worker pool
	fmt.Printf("\n🚀 Starting %d auctions with %d bidders each...\n\n", cfg.NumAuctions, cfg.NumBidders)

	overallStart := time.Now()

	results := make([]*models.AuctionResult, cfg.NumAuctions)
	var mu sync.Mutex
	var wg sync.WaitGroup

	ctx := context.Background()

	// Launch ALL auctions concurrently (at the same time) as required.
	// GOMAXPROCS already constrains how many OS threads are active,
	// so we don't need a semaphore to throttle auction starts.
	for i := 0; i < cfg.NumAuctions; i++ {
		wg.Add(1)
		go func(auctionID int) {
			defer wg.Done()

			// Run the auction
			result := engine.RunAuction(ctx, auctionID, cfg.NumBidders, cfg.NumAttributes, cfg.AuctionTimeout)

			// Write per-auction output file
			if err := output.WriteResult(resultsDir, result); err != nil {
				fmt.Printf("  ⚠ Error writing auction %d result: %v\n", auctionID, err)
			}

			// Store result
			mu.Lock()
			results[auctionID] = result
			mu.Unlock()
		}(i)
	}

	wg.Wait()
	overallEnd := time.Now()

	// 5. Write summary
	timeoutMs := int(cfg.AuctionTimeout.Milliseconds())
	if err := output.WriteSummary(resultsDir, results, overallStart, overallEnd, cfg.MaxCPU, cfg.MaxMemoryMB, timeoutMs); err != nil {
		fmt.Printf("⚠ Error writing summary: %v\n", err)
	}

	// 6. Print final report
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
