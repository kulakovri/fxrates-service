package domain

import "time"

type Quote struct {
	Pair      Pair
	Price     float64
	UpdatedAt time.Time
}
