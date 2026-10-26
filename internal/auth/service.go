package auth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/eleventravel/eleventravel-api/internal/http/response"
	"github.com/eleventravel/eleventravel-api/internal/platform/auth/supabaseadmin"
	"github.com/eleventravel/eleventravel-api/internal/users"
)

type Settings struct {
	Enabled            bool
	AccessCodeRequired bool
}

type RegisterInput struct {
	Email      string
	Password   string
	Username   string
	AccessCode string
}

type RegisterResult struct {
	UserID   string
	Username string
	Email    string
}

type ProfileProvisioner interface {
	IsUsernameAvailable(ctx context.Context, username string) (bool, error)
	CreateRegisteredProfile(ctx context.Context, userID uuid.UUID, username string) error
}

type AccountProvisioner interface {
	CreateUser(ctx context.Context, input supabaseadmin.CreateUserInput) (string, error)
	DeleteUser(ctx context.Context, userID string) error
}

type Service struct {
	settings   Settings
	accessCode string
	profiles   ProfileProvisioner
	accounts   AccountProvisioner
}

func NewService(settings Settings, accessCode string, profiles ProfileProvisioner, accounts AccountProvisioner) *Service {
	return &Service{
		settings:   settings,
		accessCode: accessCode,
		profiles:   profiles,
		accounts:   accounts,
	}
}

func (s *Service) Settings() Settings {
	return s.settings
}

func (s *Service) Register(ctx context.Context, input RegisterInput) (RegisterResult, error) {
	if !s.settings.Enabled {
		return RegisterResult{}, ErrRegistrationDisabled
	}

	normalized, err := validateRegisterInput(input, s.settings.AccessCodeRequired)
	if err != nil {
		return RegisterResult{}, err
	}

	if s.settings.AccessCodeRequired && !accessCodeMatches(normalized.AccessCode, s.accessCode) {
		return RegisterResult{}, ErrInvalidAccessCode
	}

	available, err := s.profiles.IsUsernameAvailable(ctx, normalized.Username)
	if err != nil {
		return RegisterResult{}, fmt.Errorf("check username availability: %w", err)
	}
	if !available {
		return RegisterResult{}, ErrUsernameTaken
	}

	authUserID, err := s.accounts.CreateUser(ctx, supabaseadmin.CreateUserInput{
		Email:    normalized.Email,
		Password: normalized.Password,
		Username: normalized.Username,
	})
	if err != nil {
		return RegisterResult{}, mapAccountError(err)
	}

	userID, err := uuid.Parse(authUserID)
	if err != nil {
		return RegisterResult{}, s.rollbackAccount(ctx, authUserID, fmt.Errorf("%w: invalid user id from auth provider", ErrAuthProvider))
	}

	if err := s.profiles.CreateRegisteredProfile(ctx, userID, normalized.Username); err != nil {
		if errors.Is(err, users.ErrConflict) {
			// The username was taken between the availability check and the insert.
			return RegisterResult{}, s.rollbackAccount(ctx, authUserID, ErrUsernameTaken)
		}
		return RegisterResult{}, s.rollbackAccount(ctx, authUserID, fmt.Errorf("create profile: %w", err))
	}

	return RegisterResult{
		UserID:   userID.String(),
		Username: normalized.Username,
		Email:    normalized.Email,
	}, nil
}

// rollbackAccount deletes the auth user created for a failed registration and returns the cause.
// If the deletion also fails, both errors are returned so the orphaned account is visible in logs.
func (s *Service) rollbackAccount(ctx context.Context, authUserID string, cause error) error {
	if err := s.accounts.DeleteUser(context.WithoutCancel(ctx), authUserID); err != nil {
		return errors.Join(cause, fmt.Errorf("rollback auth user %s: %w", authUserID, err))
	}
	return cause
}

func mapAccountError(err error) error {
	switch {
	case errors.Is(err, supabaseadmin.ErrEmailAlreadyRegistered):
		return ErrEmailTaken
	case errors.Is(err, supabaseadmin.ErrUserRejected):
		message := "the auth provider rejected the registration data"
		var rejected *supabaseadmin.RejectedError
		if errors.As(err, &rejected) && rejected.Message != "" {
			message = rejected.Message
		}
		return ValidationError{Details: []response.ErrorDetail{{Message: message}}}
	default:
		return fmt.Errorf("%w: %v", ErrAuthProvider, err)
	}
}

// accessCodeMatches compares hashes so the check runs in constant time regardless of the code length.
func accessCodeMatches(provided string, expected string) bool {
	providedHash := sha256.Sum256([]byte(provided))
	expectedHash := sha256.Sum256([]byte(expected))
	return subtle.ConstantTimeCompare(providedHash[:], expectedHash[:]) == 1
}
