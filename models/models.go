package models

import "time"

// Attribute represents one of 20 properties of the auctioned object.
type Attribute struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

// Bid represents a single bidder's response to an auction.
type Bid struct {
	BidderID int       `json:"bidder_id"`
	Amount   float64   `json:"amount"`
	Time     time.Time `json:"time"`
}

// AuctionResult captures the full outcome of a single auction.
type AuctionResult struct {
	AuctionID  int           `json:"auction_id"`
	Attributes []Attribute   `json:"attributes"`
	TotalBids  int           `json:"total_bids"`
	Bids       []Bid         `json:"bids"`
	Winner     *Bid          `json:"winner"`
	StartTime  time.Time     `json:"start_time"`
	EndTime    time.Time     `json:"end_time"`
	Duration   time.Duration `json:"duration_ns"`
	DurationMs float64       `json:"duration_ms"`
	TimedOut   bool          `json:"timed_out"`
}
