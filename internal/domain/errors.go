package domain

import "errors"

var (
	ErrNotFound        = errors.New("not found")
	ErrUnsupportedPair = errors.New("unsupported pair")
)
