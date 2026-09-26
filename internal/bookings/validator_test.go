package bookings

import (
	"github.com/google/uuid"
	"testing"
)

func TestValidateCreateBookingRequestRequiresTitleAndType(t *testing.T) {
	t.Parallel()

	_, err := validateCreateBookingRequest(CreateBookingRequest{})
	if err == nil {
		t.Fatal("expected validation error")
	}

	var validationErr ValidationError
	if !asValidationError(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if len(validationErr.Details) == 0 {
		t.Fatal("expected validation details")
	}
}

func TestValidateCreateBookingRequestAcceptsValidPayload(t *testing.T) {
	t.Parallel()

	input, err := validateCreateBookingRequest(CreateBookingRequest{
		Title:       "Hotel Central",
		Type:        TypeHotel,
		ExternalURL: stringPointer("https://example.com/booking"),
		OccursAt:    stringPointer("2026-09-01T15:00:00Z"),
		Location:    stringPointer("Madrid"),
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if input.Status != StatusProposal {
		t.Fatalf("expected default status proposal, got %s", input.Status)
	}
}

func TestValidateExternalURLRejectsInvalidScheme(t *testing.T) {
	t.Parallel()

	_, err := validateExternalURL(stringPointer("ftp://example.com/file"))
	if err == nil {
		t.Fatal("expected error for invalid scheme")
	}
}

func TestValidateCreateBookingRequestKeepsPaymentsSeparateFromParticipants(t *testing.T) {
	t.Parallel()
	userID := uuid.New()
	input, err := validateCreateBookingRequest(CreateBookingRequest{
		Title: "Hotel", Type: TypeHotel, ParticipantUserIDs: []uuid.UUID{uuid.New()},
		Payments: []PaymentRequest{{UserID: userID, AmountCents: 30000, PaidAt: stringPointer("2026-09-01T15:00:00Z")}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(input.ParticipantUserIDs) != 1 || len(input.Payments) != 1 || input.Payments[0].UserID != userID {
		t.Fatal("expected participants and payments to remain separate")
	}
}

func TestValidateCreateBookingRequestRejectsInvalidPayment(t *testing.T) {
	t.Parallel()
	_, err := validateCreateBookingRequest(CreateBookingRequest{Title: "Hotel", Type: TypeHotel, Payments: []PaymentRequest{{UserID: uuid.New(), AmountCents: 0}}})
	if err == nil {
		t.Fatal("expected payment validation error")
	}
}

func asValidationError(err error, target *ValidationError) bool {
	validationErr, ok := err.(ValidationError)
	if !ok {
		return false
	}
	*target = validationErr
	return true
}
