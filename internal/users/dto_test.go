package users

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestProfileResponseIncludesLocationVisitDate(t *testing.T) {
	visitedAt := time.Date(2025, time.September, 20, 0, 0, 0, 0, time.UTC)
	profile := toProfileResponse(UserProfileView{
		MapPins: []UserLocationPin{{
			LocationID: uuid.New(),
			VisitedAt:  &visitedAt,
		}},
	})

	payload, err := json.Marshal(profile)
	if err != nil {
		t.Fatalf("marshal profile response: %v", err)
	}

	var decoded struct {
		MapPins []struct {
			VisitedAt *time.Time `json:"visited_at"`
		} `json:"map_pins"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal profile response: %v", err)
	}
	if len(decoded.MapPins) != 1 {
		t.Fatalf("expected one location pin, got %d", len(decoded.MapPins))
	}
	if decoded.MapPins[0].VisitedAt == nil || !decoded.MapPins[0].VisitedAt.Equal(visitedAt) {
		t.Fatalf("expected visited_at %s, got %v", visitedAt, decoded.MapPins[0].VisitedAt)
	}
}
