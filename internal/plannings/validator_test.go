package plannings

import (
	"errors"
	"testing"
)

func TestValidateCreatePlanningRequestStatusAndParticipants(t *testing.T) {
	t.Run("defaults status to draft", func(t *testing.T) {
		input, err := validateCreatePlanningRequest(CreatePlanningRequest{Name: "Trip"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if input.Status != StatusDraft {
			t.Fatalf("expected status %q, got %q", StatusDraft, input.Status)
		}
	})

	t.Run("accepts explicit status and participants by email", func(t *testing.T) {
		status := StatusConfirmed
		input, err := validateCreatePlanningRequest(CreatePlanningRequest{
			Name:   "Trip",
			Status: &status,
			Participants: []AddPlanningMemberRequest{
				{Email: "Ana@Example.com", Role: RoleMember},
				{Email: "luis@example.com", Role: RoleAdmin},
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if input.Status != StatusConfirmed {
			t.Fatalf("expected status %q, got %q", StatusConfirmed, input.Status)
		}
		if len(input.Participants) != 2 {
			t.Fatalf("expected 2 participants, got %d", len(input.Participants))
		}
		if input.Participants[0].Email != "ana@example.com" {
			t.Fatalf("expected normalized email, got %q", input.Participants[0].Email)
		}
	})

	t.Run("rejects invalid status and participant owner role", func(t *testing.T) {
		invalidStatus := "archived"
		_, err := validateCreatePlanningRequest(CreatePlanningRequest{
			Name:   "Trip",
			Status: &invalidStatus,
			Participants: []AddPlanningMemberRequest{
				{Email: "ana@example.com", Role: RoleOwner},
			},
		})

		var validationErr ValidationError
		if !errors.As(err, &validationErr) {
			t.Fatalf("expected ValidationError, got %v", err)
		}

		fields := make(map[string]bool)
		for _, detail := range validationErr.Details {
			fields[detail.Field] = true
		}
		if !fields["status"] {
			t.Fatalf("expected status detail, got %+v", validationErr.Details)
		}
		if !fields["participants[0].role"] {
			t.Fatalf("expected participants[0].role detail, got %+v", validationErr.Details)
		}
	})
}
