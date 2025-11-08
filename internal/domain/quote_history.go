package domain

import "time"

type QuoteHistory struct {
	ID         int64
	Pair       Pair
	Price      float64
	QuotedAt   time.Time
	Source     string
	UpdateID   *string
	InsertedAt time.Time
}
