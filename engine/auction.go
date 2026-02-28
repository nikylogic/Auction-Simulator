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

// attributeNames are example names for the 20 object attributes.
var attributeNames = []string{
	"durability", "weight", "color_grade", "rarity", "age",
	"craftsmanship", "material_quality", "provenance", "condition", "size",
	"aesthetic_appeal", "historical_value", "market_demand", "authenticity_score", "uniqueness",
	"brand_reputation", "functionality", "eco_friendliness", "cultural_significance", "innovation",
}

// RunAuction executes a single auction: generates attributes, fans out to
// bidders, collects bids within the timeout, and declares a winner.
func RunAuction(ctx context.Context, auctionID int, numBidders, numAttributes int, timeout time.Duration) *models.AuctionResult {
	startTime := time.Now()
	rng := rand.New(rand.NewSource(time.Now().UnixNano() + int64(auctionID)))

	// 1. Generate random attributes for the auctioned object
	attrs := generateAttributes(rng, numAttributes)

	// 2. Create a child context with the auction timeout
	auctionCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 3. Fan out to all bidders and collect bids
	bidsCh := make(chan *models.Bid, numBidders)
	var wg sync.WaitGroup

	for i := 0; i < numBidders; i++ {
		wg.Add(1)
		go func(bidderID int) {
			defer wg.Done()
			// Each bidder gets its own RNG seeded from auction+bidder IDs
			bidderRng := rand.New(rand.NewSource(time.Now().UnixNano() + int64(bidderID*1000+auctionID)))
			bid := bidder.BidOnAuction(auctionCtx, bidderID, attrs, bidderRng)
			if bid != nil {
				bidsCh <- bid
			}
		}(i)
	}

	// Close the channel once all bidders finish (or time out)
	go func() {
		wg.Wait()
		close(bidsCh)
	}()

	// 4. Collect all bids
	var bids []models.Bid
	for bid := range bidsCh {
		bids = append(bids, *bid)
	}

	endTime := time.Now()
	duration := endTime.Sub(startTime)

	// 5. Determine winner (highest bid)
	var winner *models.Bid
	for i := range bids {
		if winner == nil || bids[i].Amount > winner.Amount {
			w := bids[i] // copy
			winner = &w
		}
	}

	// Check if we timed out (context deadline exceeded)
	timedOut := auctionCtx.Err() != nil

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

	// Log a one-liner for this auction
	winnerInfo := "no bids"
	if winner != nil {
		winnerInfo = fmt.Sprintf("winner=Bidder#%d amount=$%.2f", winner.BidderID, winner.Amount)
	}
	fmt.Printf("  [Auction %2d] %d bids | %s | %.1fms | timeout=%v\n",
		auctionID, len(bids), winnerInfo, result.DurationMs, timedOut)

	return result
}

// generateAttributes creates N random attributes with values in [0, 1].
func generateAttributes(rng *rand.Rand, n int) []models.Attribute {
	attrs := make([]models.Attribute, n)
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("attr_%d", i)
		if i < len(attributeNames) {
			name = attributeNames[i]
		}
		attrs[i] = models.Attribute{
			Name:  name,
			Value: rng.Float64(),
		}
	}
	return attrs
}
