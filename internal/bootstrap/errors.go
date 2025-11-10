package bootstrap

import "errors"

var ErrMissingDBURL = errors.New("DATABASE_URL is required")
