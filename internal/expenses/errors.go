package expenses

import (
	"errors"

	"github.com/eleventravel/eleventravel-api/internal/http/response"
)

var (
	ErrExpenseNotFound = errors.New("expense not found")
	ErrForbidden       = errors.New("forbidden")
)

type ValidationError struct {
	Details []response.ErrorDetail
}

func (e ValidationError) Error() string {
	return "validation error"
}
