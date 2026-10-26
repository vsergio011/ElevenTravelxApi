package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eleventravel/eleventravel-api/internal/platform/auth/supabaseadmin"
	"github.com/eleventravel/eleventravel-api/internal/users"
)

const testAccessCode = "beta-2025"

type fakeProfiles struct {
	available     bool
	availableErr  error
	createErr     error
	createCalls   int
	createdUserID uuid.UUID
	createdName   string
}

func (f *fakeProfiles) IsUsernameAvailable(_ context.Context, _ string) (bool, error) {
	return f.available, f.availableErr
}

func (f *fakeProfiles) CreateRegisteredProfile(_ context.Context, userID uuid.UUID, username string) error {
	f.createCalls++
	f.createdUserID = userID
	f.createdName = username
	return f.createErr
}

type fakeAccounts struct {
	authUserID  string
	createErr   error
	deleteErr   error
	createCalls int
	deleted     []string
	lastInput   supabaseadmin.CreateUserInput
}

func (f *fakeAccounts) CreateUser(_ context.Context, input supabaseadmin.CreateUserInput) (string, error) {
	f.createCalls++
	f.lastInput = input
	return f.authUserID, f.createErr
}

func (f *fakeAccounts) DeleteUser(_ context.Context, userID string) error {
	f.deleted = append(f.deleted, userID)
	return f.deleteErr
}

func newTestService(settings Settings, profiles *fakeProfiles, accounts *fakeAccounts) *Service {
	return NewService(settings, testAccessCode, profiles, accounts)
}

func validInput() RegisterInput {
	return RegisterInput{
		Email:      " Ana@Example.com ",
		Password:   "SuperSecret1!",
		Username:   "  Ana_Travel ",
		AccessCode: testAccessCode,
	}
}

func openSettings() Settings {
	return Settings{Enabled: true, AccessCodeRequired: false}
}

func restrictedSettings() Settings {
	return Settings{Enabled: true, AccessCodeRequired: true}
}

func TestRegisterRejectsWhenDisabled(t *testing.T) {
	profiles := &fakeProfiles{available: true}
	accounts := &fakeAccounts{authUserID: uuid.NewString()}
	service := newTestService(Settings{Enabled: false}, profiles, accounts)

	_, err := service.Register(context.Background(), validInput())

	assert.ErrorIs(t, err, ErrRegistrationDisabled)
	assert.Equal(t, 0, accounts.createCalls)
	assert.Equal(t, 0, profiles.createCalls)
}

func TestRegisterRejectsWrongAccessCode(t *testing.T) {
	profiles := &fakeProfiles{available: true}
	accounts := &fakeAccounts{authUserID: uuid.NewString()}
	service := newTestService(restrictedSettings(), profiles, accounts)

	input := validInput()
	input.AccessCode = "wrong-code"
	_, err := service.Register(context.Background(), input)

	assert.ErrorIs(t, err, ErrInvalidAccessCode)
	assert.Equal(t, 0, accounts.createCalls)
}

func TestRegisterRequiresAccessCodeWhenRestricted(t *testing.T) {
	profiles := &fakeProfiles{available: true}
	accounts := &fakeAccounts{authUserID: uuid.NewString()}
	service := newTestService(restrictedSettings(), profiles, accounts)

	input := validInput()
	input.AccessCode = "   "
	_, err := service.Register(context.Background(), input)

	var validationErr ValidationError
	require.ErrorAs(t, err, &validationErr)
	require.Len(t, validationErr.Details, 1)
	assert.Equal(t, "access_code", validationErr.Details[0].Field)
	assert.Equal(t, 0, accounts.createCalls)
}

func TestRegisterIgnoresAccessCodeWhenNotRestricted(t *testing.T) {
	userID := uuid.New()
	profiles := &fakeProfiles{available: true}
	accounts := &fakeAccounts{authUserID: userID.String()}
	service := newTestService(openSettings(), profiles, accounts)

	input := validInput()
	input.AccessCode = ""
	result, err := service.Register(context.Background(), input)

	require.NoError(t, err)
	assert.Equal(t, userID.String(), result.UserID)
}

func TestRegisterValidatesInput(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*RegisterInput)
		field  string
	}{
		{name: "invalid email", mutate: func(i *RegisterInput) { i.Email = "not-an-email" }, field: "email"},
		{name: "display name email", mutate: func(i *RegisterInput) { i.Email = "Ana <ana@example.com>" }, field: "email"},
		{name: "short password", mutate: func(i *RegisterInput) { i.Password = "short" }, field: "password"},
		{name: "weak password", mutate: func(i *RegisterInput) { i.Password = "weakpassword" }, field: "password"},
		{name: "long password", mutate: func(i *RegisterInput) { i.Password = string(make([]byte, 73)) }, field: "password"},
		{name: "invalid username", mutate: func(i *RegisterInput) { i.Username = "a b" }, field: "username"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			profiles := &fakeProfiles{available: true}
			accounts := &fakeAccounts{authUserID: uuid.NewString()}
			service := newTestService(openSettings(), profiles, accounts)

			input := validInput()
			test.mutate(&input)
			_, err := service.Register(context.Background(), input)

			var validationErr ValidationError
			require.ErrorAs(t, err, &validationErr)
			fields := make([]string, 0, len(validationErr.Details))
			for _, detail := range validationErr.Details {
				fields = append(fields, detail.Field)
			}
			assert.Contains(t, fields, test.field)
			assert.Equal(t, 0, accounts.createCalls)
		})
	}
}

func TestRegisterRejectsTakenUsernameBeforeCreatingAccount(t *testing.T) {
	profiles := &fakeProfiles{available: false}
	accounts := &fakeAccounts{authUserID: uuid.NewString()}
	service := newTestService(openSettings(), profiles, accounts)

	_, err := service.Register(context.Background(), validInput())

	assert.ErrorIs(t, err, ErrUsernameTaken)
	assert.Equal(t, 0, accounts.createCalls)
}

func TestRegisterMapsEmailAlreadyRegistered(t *testing.T) {
	profiles := &fakeProfiles{available: true}
	accounts := &fakeAccounts{createErr: supabaseadmin.ErrEmailAlreadyRegistered}
	service := newTestService(openSettings(), profiles, accounts)

	_, err := service.Register(context.Background(), validInput())

	assert.ErrorIs(t, err, ErrEmailTaken)
	assert.Equal(t, 0, profiles.createCalls)
}

func TestRegisterMapsProviderRejection(t *testing.T) {
	profiles := &fakeProfiles{available: true}
	accounts := &fakeAccounts{createErr: &supabaseadmin.RejectedError{Message: "Password is too common"}}
	service := newTestService(openSettings(), profiles, accounts)

	_, err := service.Register(context.Background(), validInput())

	var validationErr ValidationError
	require.ErrorAs(t, err, &validationErr)
	require.Len(t, validationErr.Details, 1)
	assert.Equal(t, "Password is too common", validationErr.Details[0].Message)
}

func TestRegisterMapsUnexpectedProviderError(t *testing.T) {
	profiles := &fakeProfiles{available: true}
	accounts := &fakeAccounts{createErr: errors.New("connection reset")}
	service := newTestService(openSettings(), profiles, accounts)

	_, err := service.Register(context.Background(), validInput())

	assert.ErrorIs(t, err, ErrAuthProvider)
}

func TestRegisterCreatesAccountAndProfile(t *testing.T) {
	userID := uuid.New()
	profiles := &fakeProfiles{available: true}
	accounts := &fakeAccounts{authUserID: userID.String()}
	service := newTestService(restrictedSettings(), profiles, accounts)

	result, err := service.Register(context.Background(), validInput())

	require.NoError(t, err)
	assert.Equal(t, userID.String(), result.UserID)
	assert.Equal(t, "ana_travel", result.Username)
	assert.Equal(t, "ana@example.com", result.Email)

	assert.Equal(t, "ana@example.com", accounts.lastInput.Email)
	assert.Equal(t, "SuperSecret1!", accounts.lastInput.Password)
	assert.Equal(t, "ana_travel", accounts.lastInput.Username)

	assert.Equal(t, 1, profiles.createCalls)
	assert.Equal(t, userID, profiles.createdUserID)
	assert.Equal(t, "ana_travel", profiles.createdName)
	assert.Empty(t, accounts.deleted)
}

func TestRegisterRollsBackAccountWhenUsernameRaceConflicts(t *testing.T) {
	userID := uuid.NewString()
	profiles := &fakeProfiles{available: true, createErr: users.ErrConflict}
	accounts := &fakeAccounts{authUserID: userID}
	service := newTestService(openSettings(), profiles, accounts)

	_, err := service.Register(context.Background(), validInput())

	assert.ErrorIs(t, err, ErrUsernameTaken)
	assert.Equal(t, []string{userID}, accounts.deleted)
}

func TestRegisterRollsBackAccountWhenProfileFails(t *testing.T) {
	userID := uuid.NewString()
	profiles := &fakeProfiles{available: true, createErr: errors.New("db down")}
	accounts := &fakeAccounts{authUserID: userID}
	service := newTestService(openSettings(), profiles, accounts)

	_, err := service.Register(context.Background(), validInput())

	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrUsernameTaken)
	assert.Equal(t, []string{userID}, accounts.deleted)
}

func TestRegisterReportsFailedRollback(t *testing.T) {
	userID := uuid.NewString()
	profiles := &fakeProfiles{available: true, createErr: errors.New("db down")}
	accounts := &fakeAccounts{authUserID: userID, deleteErr: errors.New("admin api unavailable")}
	service := newTestService(openSettings(), profiles, accounts)

	_, err := service.Register(context.Background(), validInput())

	require.Error(t, err)
	assert.ErrorContains(t, err, "db down")
	assert.ErrorContains(t, err, "admin api unavailable")
}
