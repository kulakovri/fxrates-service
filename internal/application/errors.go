package application

import "errors"

var ErrNotFound = errors.New("not found")
var ErrConflict = errors.New("conflict")
var ErrBadRequest = errors.New("bad request")
var ErrUnsupportedPair = errors.New("unsupported pair")
