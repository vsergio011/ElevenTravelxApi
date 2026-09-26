package checklists

import (
	"errors"

	"github.com/eleventravel/eleventravel-api/internal/http/response"
)

var (
	ErrTaskNotFound = errors.New("checklist task not found")
	ErrForbidden    = errors.New("forbidden")
)

type ValidationError struct {
	Details []response.ErrorDetail
}

func (e ValidationError) Error() string {
	return "validation error"
}
