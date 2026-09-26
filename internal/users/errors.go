package users

import (
	"errors"

	"github.com/eleventravel/eleventravel-api/internal/http/response"
)

var (
	ErrProfileNotFound       = errors.New("profile not found")
	ErrForbidden             = errors.New("forbidden")
	ErrConflict              = errors.New("conflict")
	ErrNotFound              = errors.New("not found")
	ErrLocationNotFound      = errors.New("location not found")
	ErrBadgeNotFound         = errors.New("badge not found")
	ErrBadgeUnavailable      = errors.New("badge unavailable")
	ErrBadgeLocationMismatch = errors.New("badge location mismatch")
	ErrInvalidPrivacyValue   = errors.New("invalid privacy value")
)

type ValidationError struct {
	Details []response.ErrorDetail
}

func (e ValidationError) Error() string {
	return "validation error"
}
