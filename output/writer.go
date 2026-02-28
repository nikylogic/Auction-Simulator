// Package output writes auction results to JSON files.
// After each auction finishes, this package saves the full details to a file.
// At the very end, it also creates a summary file with stats from all 40 auctions.
package output

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/auction-simulator/models"
)

// Summary is the big-picture view of the entire simulation run.
// It answers questions like: "How many auctions ran? How many bids total?
// How long did the whole thing take? What resources were we using?"
type Summary struct {
	TotalAuctions     int            `json:"total_auctions"`          // how many auctions ran (should be 40)
	TotalBidsAll      int            `json:"total_bids_all_auctions"` // total bids across ALL auctions combined
	AvgBidsPerAuction float64        `json:"avg_bids_per_auction"`    // average number of bids per auction
	AuctionsTimedOut  int            `json:"auctions_timed_out"`      // how many auctions hit the timeout
	OverallStartTime  time.Time      `json:"overall_start_time"`      // when the first auction started
	OverallEndTime    time.Time      `json:"overall_end_time"`        // when the last auction finished
	TotalDurationMs   float64        `json:"total_duration_ms"`       // total wall-clock time in milliseconds
	ResourceConfig    ResourceConfig `json:"resource_config"`         // what CPU/memory limits were active
	AuctionSummaries  []AuctionBrief `json:"auction_summaries"`       // a quick summary of each individual auction
}

// ResourceConfig records what resource limits were used during this run.
// This is important for benchmarking — so you know exactly what constraints
// were active when looking at the results later.
type ResourceConfig struct {
	VCPU      int   `json:"vcpu"`       // how many CPU threads were allowed
	MemoryMB  int64 `json:"memory_mb"`  // memory limit in megabytes
	TimeoutMs int   `json:"timeout_ms"` // auction timeout in milliseconds
}

// AuctionBrief is a quick snapshot of one auction's result.
// Instead of storing all the bids and attributes (which can be huge),
// this just captures the key numbers — who won, how much, how long it took.
type AuctionBrief struct {
	AuctionID  int      `json:"auction_id"`  // which auction (0-39)
	TotalBids  int      `json:"total_bids"`  // how many bids were received
	WinnerID   *int     `json:"winner_id"`   // who won (nil if nobody bid)
	WinAmount  *float64 `json:"win_amount"`  // how much the winner bid (nil if nobody bid)
	DurationMs float64  `json:"duration_ms"` // how long the auction took in milliseconds
	TimedOut   bool     `json:"timed_out"`   // did this auction hit the timeout?
}

// WriteResult saves one auction's full result to a JSON file.
// For example, auction 0 gets saved as "results/auction_0.json".
// The file contains everything — all 20 attributes, every bid, the winner, timing info.
func WriteResult(dir string, result *models.AuctionResult) error {
	// Create the output directory if it doesn't exist yet
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	// Build the filename: "results/auction_0.json", "results/auction_1.json", etc.
	filename := filepath.Join(dir, fmt.Sprintf("auction_%d.json", result.AuctionID))

	// Convert the result struct to nicely formatted JSON (indented with 2 spaces)
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal auction %d: %w", result.AuctionID, err)
	}

	// Write the JSON to the file
	if err := os.WriteFile(filename, data, 0o644); err != nil {
		return fmt.Errorf("write auction %d: %w", result.AuctionID, err)
	}

	return nil
}

// WriteSummary creates the "summary.json" file after ALL auctions have finished.
// It loops through every result, adds up the totals, and writes the aggregate stats.
func WriteSummary(dir string, results []*models.AuctionResult, overallStart, overallEnd time.Time, vcpu int, memMB int64, timeoutMs int) error {
	// Create the output directory if it doesn't exist yet
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	totalBids := 0
	timedOut := 0
	briefs := make([]AuctionBrief, len(results))

	// Loop through all 40 results and build a compact summary for each
	for i, r := range results {
		totalBids += r.TotalBids
		if r.TimedOut {
			timedOut++
		}

		// Create a brief summary for this auction
		brief := AuctionBrief{
			AuctionID:  r.AuctionID,
			TotalBids:  r.TotalBids,
			DurationMs: r.DurationMs,
			TimedOut:   r.TimedOut,
		}
		// If there was a winner, include their info
		if r.Winner != nil {
			id := r.Winner.BidderID
			amt := r.Winner.Amount
			brief.WinnerID = &id
			brief.WinAmount = &amt
		}
		briefs[i] = brief
	}

	// Calculate the average number of bids per auction
	avgBids := 0.0
	if len(results) > 0 {
		avgBids = float64(totalBids) / float64(len(results))
	}

	// Build the final summary object
	summary := Summary{
		TotalAuctions:     len(results),
		TotalBidsAll:      totalBids,
		AvgBidsPerAuction: avgBids,
		AuctionsTimedOut:  timedOut,
		OverallStartTime:  overallStart,
		OverallEndTime:    overallEnd,
		TotalDurationMs:   float64(overallEnd.Sub(overallStart).Microseconds()) / 1000.0,
		ResourceConfig: ResourceConfig{
			VCPU:      vcpu,
			MemoryMB:  memMB,
			TimeoutMs: timeoutMs,
		},
		AuctionSummaries: briefs,
	}

	// Convert to JSON and write to file
	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal summary: %w", err)
	}

	filename := filepath.Join(dir, "summary.json")
	if err := os.WriteFile(filename, data, 0o644); err != nil {
		return fmt.Errorf("write summary: %w", err)
	}

	return nil
}
