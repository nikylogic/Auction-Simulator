package output

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/auction-simulator/models"
)

// Summary holds aggregate statistics across all auctions.
type Summary struct {
	TotalAuctions     int            `json:"total_auctions"`
	TotalBidsAll      int            `json:"total_bids_all_auctions"`
	AvgBidsPerAuction float64        `json:"avg_bids_per_auction"`
	AuctionsTimedOut  int            `json:"auctions_timed_out"`
	OverallStartTime  time.Time      `json:"overall_start_time"`
	OverallEndTime    time.Time      `json:"overall_end_time"`
	TotalDurationMs   float64        `json:"total_duration_ms"`
	ResourceConfig    ResourceConfig `json:"resource_config"`
	AuctionSummaries  []AuctionBrief `json:"auction_summaries"`
}

// ResourceConfig records the resource constraints used.
type ResourceConfig struct {
	VCPU      int   `json:"vcpu"`
	MemoryMB  int64 `json:"memory_mb"`
	TimeoutMs int   `json:"timeout_ms"`
}

// AuctionBrief is a compact summary of a single auction.
type AuctionBrief struct {
	AuctionID  int      `json:"auction_id"`
	TotalBids  int      `json:"total_bids"`
	WinnerID   *int     `json:"winner_id"`
	WinAmount  *float64 `json:"win_amount"`
	DurationMs float64  `json:"duration_ms"`
	TimedOut   bool     `json:"timed_out"`
}

// WriteResult writes a single auction result as a JSON file.
func WriteResult(dir string, result *models.AuctionResult) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	filename := filepath.Join(dir, fmt.Sprintf("auction_%d.json", result.AuctionID))
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal auction %d: %w", result.AuctionID, err)
	}

	if err := os.WriteFile(filename, data, 0o644); err != nil {
		return fmt.Errorf("write auction %d: %w", result.AuctionID, err)
	}

	return nil
}

// WriteSummary writes the aggregate summary JSON file.
func WriteSummary(dir string, results []*models.AuctionResult, overallStart, overallEnd time.Time, vcpu int, memMB int64, timeoutMs int) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	totalBids := 0
	timedOut := 0
	briefs := make([]AuctionBrief, len(results))

	for i, r := range results {
		totalBids += r.TotalBids
		if r.TimedOut {
			timedOut++
		}

		brief := AuctionBrief{
			AuctionID:  r.AuctionID,
			TotalBids:  r.TotalBids,
			DurationMs: r.DurationMs,
			TimedOut:   r.TimedOut,
		}
		if r.Winner != nil {
			id := r.Winner.BidderID
			amt := r.Winner.Amount
			brief.WinnerID = &id
			brief.WinAmount = &amt
		}
		briefs[i] = brief
	}

	avgBids := 0.0
	if len(results) > 0 {
		avgBids = float64(totalBids) / float64(len(results))
	}

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
