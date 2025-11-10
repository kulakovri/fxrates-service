package domain

import "time"

type QuoteUpdate struct {
	ID        string
	Pair      Pair
	Status    QuoteUpdateStatus
	Error     *string
	Price     *float64
	UpdatedAt time.Time
}
