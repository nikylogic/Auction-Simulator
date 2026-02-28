// Package bidder contains the logic for a single bidder.
// Each bidder is like a person at an auction — they look at the object,
// think about whether they like it, and either place a bid or walk away.
package bidder

import (
	"context"
	"math"
	"math/rand"
	"time"

	"github.com/auction-simulator/models"
)

// BidOnAuction simulates one bidder looking at an auction object and deciding what to do.
//
// The bidder goes through three stages:
//  1. "Thinking" — wait a random amount of time (0 to 150ms), simulating real-world delay
//  2. "Evaluating" — compute an interest score based on the object's attributes
//  3. "Deciding" — if interested enough, calculate a bid amount and submit it
//
// If the auction times out while the bidder is thinking, they get kicked out
// and return nil (no bid). This is done cleanly through Go's context system.
func BidOnAuction(ctx context.Context, bidderID int, attrs []models.Attribute, rng *rand.Rand) *models.Bid {

	// --- STAGE 1: Simulate thinking time ---
	// In real life, a bidder needs time to evaluate an item.
	// We simulate this with a random delay between 0 and 150 milliseconds.
	delay := time.Duration(rng.Intn(150)) * time.Millisecond

	// The "select" is like saying: "either wait for my thinking time to finish,
	// OR if the auction ends before I'm done thinking, just leave immediately."
	select {
	case <-ctx.Done():
		// The auction closed before we finished thinking. Too late, no bid.
		return nil
	case <-time.After(delay):
		// We finished thinking in time. Continue to evaluation.
	}

	// --- STAGE 2: Evaluate the object ---
	// Each bidder has personal preferences — one bidder might care a lot about
	// "rarity" but not about "weight", while another bidder is the opposite.
	// The interest score combines the object's attributes with the bidder's preferences.
	score := computeInterestScore(bidderID, attrs, rng)

	// --- STAGE 3: Decide whether to bid ---
	// If the interest score is below 0.30, the bidder isn't interested enough.
	// On average, about 30% of bidders will decline for any given auction.
	// This satisfies the requirement: "not every bidder will provide a response."
	threshold := 0.30
	if score < threshold {
		return nil // "Nah, not for me." — bidder walks away
	}

	// --- Calculate the bid amount ---
	// The more interested the bidder is, the more they'll pay.
	// Base amount = interest score × 1000 (so a score of 0.8 → ~$800)
	// Then we add some randomness (±$50) so bids aren't perfectly predictable.
	baseAmount := score * 1000.0
	noise := (rng.Float64() - 0.5) * 100.0    // random number between -50 and +50
	amount := math.Max(1.0, baseAmount+noise) // make sure the bid is at least $1
	amount = math.Round(amount*100) / 100     // round to 2 decimal places (e.g., $823.50)

	// Submit the bid!
	return &models.Bid{
		BidderID: bidderID,
		Amount:   amount,
		Time:     time.Now(),
	}
}

// computeInterestScore figures out how much a specific bidder likes a specific object.
//
// It works like this:
//   - Each bidder has a unique set of "weights" (personal preferences)
//     For example, Bidder #5 might give high weight to "rarity" but low weight to "size"
//   - We multiply each attribute's value by the bidder's weight for that attribute
//   - Then we average it all together to get a score between 0 and 1
//   - A small random jitter (±0.05) is added so the same bidder doesn't always
//     score identically across different auctions
//
// The weights are deterministic per bidder (seeded by bidder ID),
// so Bidder #5 always has the same preferences regardless of which auction they're in.
func computeInterestScore(bidderID int, attrs []models.Attribute, rng *rand.Rand) float64 {
	// Create a separate random generator just for this bidder's weights.
	// Seeded by bidder ID so the same bidder always has the same preferences.
	weightRng := rand.New(rand.NewSource(int64(bidderID * 31337)))

	var weightedSum float64
	var totalWeight float64

	// For each attribute of the object, get this bidder's weight for it
	// and add (attribute value × weight) to the running total.
	for _, attr := range attrs {
		weight := weightRng.Float64() // this bidder's personal weight for this attribute
		weightedSum += attr.Value * weight
		totalWeight += weight
	}

	// Edge case: if total weight is somehow zero, just return 0
	if totalWeight == 0 {
		return 0
	}

	// Normalize to a 0-to-1 range, then add a tiny bit of randomness
	// so there's some variation between auctions (real people aren't perfectly consistent)
	normalized := weightedSum / totalWeight
	jitter := (rng.Float64() - 0.5) * 0.1                // small random nudge: ±0.05
	score := math.Max(0, math.Min(1, normalized+jitter)) // clamp between 0 and 1

	return score
}
