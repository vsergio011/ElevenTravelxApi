package flights

import (
	"testing"
	"time"
)

func TestValidateSegmentsRequiresChronologicalPositions(t *testing.T) {
	base := time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC)
	segments := []Segment{
		{Direction: DirectionOutbound, Position: 0, OriginLabel: "MAD", DestinationLabel: "CDG", DepartureAt: base, ArrivalAt: base.Add(2 * time.Hour)},
		{Direction: DirectionOutbound, Position: 1, OriginLabel: "CDG", DestinationLabel: "NRT", DepartureAt: base.Add(90 * time.Minute), ArrivalAt: base.Add(12 * time.Hour)},
	}
	if err := validateSegments(segments); err == nil {
		t.Fatal("expected overlapping segments to be rejected")
	}

	segments[1].DepartureAt = base.Add(3 * time.Hour)
	if err := validateSegments(segments); err != nil {
		t.Fatalf("expected chronological segments, got %v", err)
	}
}

func TestValidateInputRejectsInvalidMoneyAndDistribution(t *testing.T) {
	base := time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC)
	err := validateInput(CreateInput{
		Status: StatusDraft, Currency: "EUR", CostCents: -1, CostDistribution: "unknown",
		Segments: []Segment{{Direction: DirectionOutbound, Position: 0, OriginLabel: "MAD", DestinationLabel: "CDG", DepartureAt: base, ArrivalAt: base.Add(time.Hour)}},
	})
	if err == nil {
		t.Fatal("expected invalid money and distribution to be rejected")
	}
}

func TestValidateInputAllowsOneWayFlight(t *testing.T) {
	base := time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC)
	err := validateInput(CreateInput{
		Status: StatusDraft, Currency: "EUR", CostCents: 12550, CostDistribution: DistributionGroup,
		Segments: []Segment{{Direction: DirectionOutbound, Position: 0, OriginLabel: "MAD", DestinationLabel: "CDG", DepartureAt: base, ArrivalAt: base.Add(time.Hour)}},
	})
	if err != nil {
		t.Fatalf("expected one-way flight to be valid, got %v", err)
	}
}
