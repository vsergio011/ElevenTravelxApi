package plannings

import (
	"testing"

	"github.com/google/uuid"
)

func TestNewPlanningDashboardActivityResponseIncludesUpcomingActivityDetails(t *testing.T) {
	location := "Asakusa, Tokio"
	description := "Visita el templo y pasea por Nakamise-dori"
	activity := PlanningDashboardActivity{
		PlanningID: uuid.New(),
		UpcomingActivities: []PlanningUpcomingActivity{
			{
				ID:          uuid.New(),
				Title:       "Templo Senso-ji",
				StartsAt:    "2026-11-10T09:30",
				Location:    &location,
				Description: &description,
			},
		},
	}

	response := NewPlanningDashboardActivityResponse(activity)
	if len(response.UpcomingActivities) != 1 {
		t.Fatalf("expected one upcoming activity, got %d", len(response.UpcomingActivities))
	}

	upcoming := response.UpcomingActivities[0]
	if upcoming.Location == nil || *upcoming.Location != location {
		t.Fatalf("expected location %q, got %v", location, upcoming.Location)
	}
	if upcoming.Description == nil || *upcoming.Description != description {
		t.Fatalf("expected description %q, got %v", description, upcoming.Description)
	}
}
