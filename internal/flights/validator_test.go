package flights

import (
	"testing"
	"time"

	"github.com/google/uuid"
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

func TestValidateSegmentsRequiresConnectedStops(t *testing.T) {
	base := time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC)
	segments := []Segment{
		{Direction: DirectionOutbound, Position: 1, OriginLabel: "LHR", DestinationLabel: "NRT", DepartureAt: base.Add(3 * time.Hour), ArrivalAt: base.Add(15 * time.Hour)},
		{Direction: DirectionOutbound, Position: 0, OriginLabel: "MAD", DestinationLabel: "CDG", DepartureAt: base, ArrivalAt: base.Add(2 * time.Hour)},
	}
	if err := validateSegments(segments); err == nil {
		t.Fatal("expected disconnected segments to be rejected")
	}

	segments[1].DestinationLabel = "LHR"
	if err := validateSegments(segments); err != nil {
		t.Fatalf("expected connected segments in any input order to be valid, got %v", err)
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

func TestValidatePaymentsUsesCentsAndCapsAtFlightCost(t *testing.T) {
	payerID := uuid.New()
	payments := []Payment{{UserID: payerID, AmountCents: 1250}}
	if err := validatePayments(2500, payments); err != nil {
		t.Fatalf("expected a valid partial payment, got %v", err)
	}

	tests := []struct {
		name     string
		cost     int64
		payments []Payment
	}{
		{name: "zero amount", cost: 2500, payments: []Payment{{UserID: payerID}}},
		{name: "payment exceeds cost", cost: 1000, payments: []Payment{{UserID: payerID, AmountCents: 1001}}},
		{name: "combined payments exceed cost", cost: 1000, payments: []Payment{
			{UserID: payerID, AmountCents: 600},
			{UserID: uuid.New(), AmountCents: 401},
		}},
		{name: "missing payer", cost: 1000, payments: []Payment{{AmountCents: 1}}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := validatePayments(test.cost, test.payments); err == nil {
				t.Fatal("expected invalid payment list to be rejected")
			}
		})
	}
}
