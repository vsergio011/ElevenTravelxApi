package auth

import (
	"errors"

	"github.com/eleventravel/eleventravel-api/internal/http/response"
)

var (
	ErrRegistrationDisabled = errors.New("registration disabled")
	ErrInvalidAccessCode    = errors.New("invalid access code")
	ErrUsernameTaken        = errors.New("username taken")
	ErrEmailTaken           = errors.New("email taken")
	ErrAuthProvider         = errors.New("auth provider error")
)

type ValidationError struct {
	Details []response.ErrorDetail
}

func (e ValidationError) Error() string {
	return "validation error"
}
