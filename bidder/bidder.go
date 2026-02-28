package bidder

import (
	"context"
	"math"
	"math/rand"
	"time"

	"github.com/auction-simulator/models"
)

// BidOnAuction simulates a single bidder evaluating the auction attributes
// and optionally placing a bid. Returns nil if the bidder declines.
func BidOnAuction(ctx context.Context, bidderID int, attrs []models.Attribute, rng *rand.Rand) *models.Bid {
	// Simulate thinking / network latency (0–150ms)
	delay := time.Duration(rng.Intn(150)) * time.Millisecond
	select {
	case <-ctx.Done():
		return nil // auction timed out before we could respond
	case <-time.After(delay):
		// continue
	}

	// Compute interest score from attributes using per-bidder weights
	score := computeInterestScore(bidderID, attrs, rng)

	// Decline threshold: ~30% of bidders will decline on average
	threshold := 0.30
	if score < threshold {
		return nil
	}

	// Compute bid amount: base amount scaled by score + small noise
	baseAmount := score * 1000.0
	noise := (rng.Float64() - 0.5) * 100.0 // ±50
	amount := math.Max(1.0, baseAmount+noise)
	amount = math.Round(amount*100) / 100 // round to 2 decimal places

	return &models.Bid{
		BidderID: bidderID,
		Amount:   amount,
		Time:     time.Now(),
	}
}

// computeInterestScore calculates a deterministic interest score in [0, 1]
// based on the auction attributes and the bidder's unique preferences.
func computeInterestScore(bidderID int, attrs []models.Attribute, rng *rand.Rand) float64 {
	// Each bidder has different weights for different attributes
	// Seed a separate RNG for weight generation so weights are consistent per bidder
	weightRng := rand.New(rand.NewSource(int64(bidderID * 31337)))

	var weightedSum float64
	var totalWeight float64

	for _, attr := range attrs {
		weight := weightRng.Float64() // bidder-specific weight for this attribute
		weightedSum += attr.Value * weight
		totalWeight += weight
	}

	if totalWeight == 0 {
		return 0
	}

	// Normalize to [0, 1] and add small per-auction randomness
	normalized := weightedSum / totalWeight
	jitter := (rng.Float64() - 0.5) * 0.1 // ±0.05
	score := math.Max(0, math.Min(1, normalized+jitter))

	return score
}
