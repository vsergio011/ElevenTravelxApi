package auth

import (
	"net/mail"
	"strings"
	"unicode"

	"github.com/eleventravel/eleventravel-api/internal/http/response"
	"github.com/eleventravel/eleventravel-api/internal/users"
)

const (
	maxEmailLength     = 254
	minPasswordLength  = 8
	maxPasswordLength  = 72
	maxAccessCodeBytes = 256
)

// validateRegisterInput returns the normalized input, or a ValidationError listing every invalid field.
func validateRegisterInput(input RegisterInput, accessCodeRequired bool) (RegisterInput, error) {
	normalized := RegisterInput{
		Email:      strings.ToLower(strings.TrimSpace(input.Email)),
		Password:   input.Password,
		Username:   strings.ToLower(strings.TrimSpace(input.Username)),
		AccessCode: strings.TrimSpace(input.AccessCode),
	}

	var details []response.ErrorDetail

	if normalized.Email == "" || len(normalized.Email) > maxEmailLength {
		details = append(details, response.ErrorDetail{Field: "email", Message: "email is required and must be at most 254 characters"})
	} else if address, err := mail.ParseAddress(normalized.Email); err != nil || address.Address != normalized.Email {
		details = append(details, response.ErrorDetail{Field: "email", Message: "email must be a valid email address"})
	}

	if len(input.Password) < minPasswordLength || len(input.Password) > maxPasswordLength {
		details = append(details, response.ErrorDetail{Field: "password", Message: "password must be between 8 and 72 characters"})
	} else if !isStrongPassword(input.Password) {
		details = append(details, response.ErrorDetail{Field: "password", Message: "password must include an uppercase letter, a lowercase letter, a number and a symbol"})
	}

	if !users.IsValidUsername(normalized.Username) {
		details = append(details, response.ErrorDetail{Field: "username", Message: "username must be 3-30 chars using letters, numbers, _, -, ."})
	}

	if accessCodeRequired {
		if normalized.AccessCode == "" || len(normalized.AccessCode) > maxAccessCodeBytes {
			details = append(details, response.ErrorDetail{Field: "access_code", Message: "access_code is required"})
		}
	}

	if len(details) > 0 {
		return RegisterInput{}, ValidationError{Details: details}
	}

	return normalized, nil
}

// isStrongPassword requires at least one uppercase letter, lowercase letter, digit and symbol.
func isStrongPassword(password string) bool {
	var hasUpper, hasLower, hasDigit, hasSymbol bool

	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsDigit(char):
			hasDigit = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSymbol = true
		}
	}

	return hasUpper && hasLower && hasDigit && hasSymbol
}
