// Package engine runs the actual auctions.
// Think of this as the "auctioneer" — it creates the item,
// invites all the bidders, collects their bids, and announces the winner.
package engine

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/auction-simulator/bidder"
	"github.com/auction-simulator/models"
)

// attributeNames is a list of human-readable names for object properties.
// When we create an auction object, we pick names from this list
// so the output looks nice instead of just "attr_0", "attr_1", etc.
var attributeNames = []string{
	"durability", "weight", "color_grade", "rarity", "age",
	"craftsmanship", "material_quality", "provenance", "condition", "size",
	"aesthetic_appeal", "historical_value", "market_demand", "authenticity_score", "uniqueness",
	"brand_reputation", "functionality", "eco_friendliness", "cultural_significance", "innovation",
}

// RunAuction runs one complete auction from start to finish.
//
// Here's what happens step by step:
//  1. Create a random object with 20 attributes (like rolling dice for each property)
//  2. Start a countdown timer (the timeout — once it expires, the auction closes)
//  3. Send the attributes to all 100 bidders at the same time (fan-out)
//  4. Collect whatever bids come back before the timer runs out (fan-in)
//  5. Look through all the bids and pick the highest one — that bidder wins
//
// The "ctx" parameter carries the timeout. When time's up, every bidder
// gets notified automatically through Go's context system — no manual cleanup needed.
func RunAuction(ctx context.Context, auctionID int, numBidders, numAttributes int, timeout time.Duration) *models.AuctionResult {
	startTime := time.Now()

	// Each auction gets its own random number generator so the attributes are different every time.
	// We seed it with the auction ID so results are reproducible if you want to debug.
	rng := rand.New(rand.NewSource(time.Now().UnixNano() + int64(auctionID)))

	// --- STEP 1: Create the auction object ---
	// Generate 20 random attributes (e.g., durability=0.72, rarity=0.95, ...)
	attrs := generateAttributes(rng, numAttributes)

	// --- STEP 2: Start the countdown timer ---
	// context.WithTimeout creates a "child context" that automatically cancels
	// after the specified duration (e.g., 200ms). All bidders share this context,
	// so when time's up, they ALL get notified at once.
	auctionCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel() // always clean up the context when we're done

	// --- STEP 3: Fan out to all bidders ---
	// Create a "mailbox" (channel) where bidders can drop off their bids.
	// It's buffered to numBidders so bidders don't have to wait for each other.
	bidsCh := make(chan *models.Bid, numBidders)
	var wg sync.WaitGroup // keeps track of how many bidders are still working

	for i := 0; i < numBidders; i++ {
		wg.Add(1) // "hey, one more bidder is starting"
		go func(bidderID int) {
			defer wg.Done() // "this bidder is done, whether they bid or not"

			// Each bidder gets its own random number generator
			// so their behavior is unique but reproducible.
			bidderRng := rand.New(rand.NewSource(time.Now().UnixNano() + int64(bidderID*1000+auctionID)))

			// Ask the bidder to evaluate the object and maybe place a bid.
			// They might return nil if they're not interested or if time ran out.
			bid := bidder.BidOnAuction(auctionCtx, bidderID, attrs, bidderRng)

			// Only send the bid to the channel if the bidder actually bid something
			if bid != nil {
				bidsCh <- bid
			}
		}(i)
	}

	// This goroutine waits for ALL bidders to finish, then closes the channel.
	// Closing the channel is the signal that says "no more bids are coming."
	go func() {
		wg.Wait()
		close(bidsCh)
	}()

	// --- STEP 4: Collect all the bids ---
	// Read from the channel until it's closed (meaning all bidders are done).
	// This is the "fan-in" part — many bidders, one collection point.
	var bids []models.Bid
	for bid := range bidsCh {
		bids = append(bids, *bid)
	}

	endTime := time.Now()
	duration := endTime.Sub(startTime)

	// --- STEP 5: Pick the winner ---
	// Simple: whoever bid the most money wins.
	var winner *models.Bid
	for i := range bids {
		if winner == nil || bids[i].Amount > winner.Amount {
			w := bids[i] // make a copy so we don't accidentally point to the wrong bid
			winner = &w
		}
	}

	// Did the auction time out? If the context has an error, it means the
	// deadline fired before all bidders could respond. That's totally normal.
	timedOut := auctionCtx.Err() != nil

	// Bundle everything into an AuctionResult so we can save it and report on it
	result := &models.AuctionResult{
		AuctionID:  auctionID,
		Attributes: attrs,
		TotalBids:  len(bids),
		Bids:       bids,
		Winner:     winner,
		StartTime:  startTime,
		EndTime:    endTime,
		Duration:   duration,
		DurationMs: float64(duration.Microseconds()) / 1000.0,
		TimedOut:   timedOut,
	}

	// Print a quick one-liner so we can see progress in the terminal
	winnerInfo := "no bids"
	if winner != nil {
		winnerInfo = fmt.Sprintf("winner=Bidder#%d amount=$%.2f", winner.BidderID, winner.Amount)
	}
	fmt.Printf("  [Auction %2d] %d bids | %s | %.1fms | timeout=%v\n",
		auctionID, len(bids), winnerInfo, result.DurationMs, timedOut)

	return result
}

// generateAttributes creates N random attributes for an auction object.
// Each attribute gets a friendly name (like "durability") and a random value between 0 and 1.
// Think of it like rolling a die for each property of the object.
func generateAttributes(rng *rand.Rand, n int) []models.Attribute {
	attrs := make([]models.Attribute, n)
	for i := 0; i < n; i++ {
		// Use a real name if we have one, otherwise fall back to "attr_0", "attr_1", etc.
		name := fmt.Sprintf("attr_%d", i)
		if i < len(attributeNames) {
			name = attributeNames[i]
		}
		attrs[i] = models.Attribute{
			Name:  name,
			Value: rng.Float64(), // random number between 0.0 and 1.0
		}
	}
	return attrs
}
