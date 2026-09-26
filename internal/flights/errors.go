package flights

import (
	"errors"
)

var (
	ErrFlightNotFound          = errors.New("flight not found")
	ErrFlightLookupNotFound    = errors.New("flight lookup not found")
	ErrFlightLookupUnavailable = errors.New("flight lookup unavailable")
	ErrForbidden               = errors.New("forbidden")
)
