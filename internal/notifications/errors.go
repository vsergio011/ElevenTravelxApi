package notifications

import (
	"errors"

	"github.com/eleventravel/eleventravel-api/internal/http/response"
)

var (
	ErrNotFound = errors.New("notification not found")
)

type ValidationError struct {
	Details []response.ErrorDetail
}

func (e ValidationError) Error() string {
	return "validation error"
}

