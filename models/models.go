// Package models defines the basic building blocks for the auction system.
// Think of this like defining the "shapes" of our data —
// what an auction object looks like, what a bid looks like, and what a result looks like.
// Every other package uses these types, but this package doesn't depend on anyone else.
package models

import "time"

// Attribute is one property of the object being auctioned.
// For example, if we're selling a painting, an attribute could be
// "color_grade" with a value of 0.8 (meaning it scores 80% on color quality).
// Each auction object has 20 of these.
type Attribute struct {
	Name  string  `json:"name"`  // the name of the property, like "durability" or "rarity"
	Value float64 `json:"value"` // a score between 0 and 1 — higher means better
}

// Bid is what a bidder sends back when they want to buy the object.
// Not every bidder will send one — some look at the attributes and say "not worth it."
type Bid struct {
	BidderID int       `json:"bidder_id"` // which bidder placed this bid (like a player number)
	Amount   float64   `json:"amount"`    // how much money they're offering (in dollars)
	Time     time.Time `json:"time"`      // the exact moment they placed the bid
}

// AuctionResult captures everything that happened in a single auction.
// After the auction closes, we bundle all the info into this struct
// so we can save it to a file and show it in the report.
type AuctionResult struct {
	AuctionID  int           `json:"auction_id"`  // which auction this is (0, 1, 2, ... up to 39)
	Attributes []Attribute   `json:"attributes"`  // the 20 attributes of the object that was auctioned
	TotalBids  int           `json:"total_bids"`  // how many bidders actually placed a bid
	Bids       []Bid         `json:"bids"`        // the full list of all bids received
	Winner     *Bid          `json:"winner"`      // the highest bid — this person wins (nil if nobody bid)
	StartTime  time.Time     `json:"start_time"`  // when the auction started
	EndTime    time.Time     `json:"end_time"`    // when the auction finished
	Duration   time.Duration `json:"duration_ns"` // how long the auction lasted (in nanoseconds internally)
	DurationMs float64       `json:"duration_ms"` // same duration but in milliseconds (easier to read)
	TimedOut   bool          `json:"timed_out"`   // did the auction hit the timeout before all bidders responded?
}
