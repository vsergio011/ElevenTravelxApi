package bookings

import (
	"errors"

	"github.com/eleventravel/eleventravel-api/internal/http/response"
)

var (
	ErrBookingNotFound = errors.New("booking not found")
	ErrForbidden       = errors.New("forbidden")
	ErrConflict        = errors.New("conflict")
)

type ValidationError struct {
	Details []response.ErrorDetail
}

func (e ValidationError) Error() string {
	return "validation error"
}
