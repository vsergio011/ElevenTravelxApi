package plannings

import (
	"errors"

	"github.com/eleventravel/eleventravel-api/internal/http/response"
)

var (
	ErrPlanningNotFound = errors.New("planning not found")
	ErrMemberNotFound   = errors.New("planning member not found")
	ErrUserNotFound     = errors.New("user not found")
	ErrForbidden        = errors.New("forbidden")
	ErrConflict         = errors.New("conflict")
)

type ValidationError struct {
	Details []response.ErrorDetail
}

func (e ValidationError) Error() string {
	return "validation error"
}
