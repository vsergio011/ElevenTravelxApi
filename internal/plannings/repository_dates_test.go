package plannings

import (
	"testing"
	"time"
)

func TestRouteStopStartsAtResolvesLegacyCalendarDate(t *testing.T) {
	planningStartsAt := time.Date(2026, time.November, 10, 0, 0, 0, 0, time.UTC)

	startsAt := routeStopStartsAt("10 nov", &planningStartsAt, 1, "09:30", planningStartsAt)

	if startsAt != "2026-11-10T09:30" {
		t.Fatalf("expected legacy date label to resolve to planning day, got %q", startsAt)
	}
}
