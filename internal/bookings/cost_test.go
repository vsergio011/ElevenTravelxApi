package bookings

import (
	"github.com/google/uuid"
	"testing"
)

func TestAllocateCostExcludesProposals(t *testing.T) {
	ids := []uuid.UUID{uuid.New(), uuid.New()}
	result := AllocateCost(Booking{Status: StatusProposal, CostCents: 1000, CostDistribution: DistributionIndividual}, ids)
	if len(result) != 0 {
		t.Fatalf("proposal allocated %d lines", len(result))
	}
}

func TestAllocateCostDistributesSelectedCentsDeterministically(t *testing.T) {
	first, second, excluded := uuid.New(), uuid.New(), uuid.New()
	result := AllocateCost(Booking{Status: StatusConfirmed, CostCents: 101, CostDistribution: DistributionSelectedParticipants, ParticipantUserIDs: []uuid.UUID{first, second}}, []uuid.UUID{first, second, excluded})
	if len(result) != 2 || result[0].Cents+result[1].Cents != 101 || result[0].Cents < result[1].Cents {
		t.Fatalf("unexpected allocation: %#v", result)
	}
}

func TestIndividualBudgetAccumulatesConfirmedBookings(t *testing.T) {
	id := uuid.New()
	result := IndividualBudget([]Booking{{Status: StatusProposal, CostCents: 999, CostDistribution: DistributionIndividual}, {Status: StatusConfirmed, CostCents: 250, CostDistribution: DistributionIndividual}}, []uuid.UUID{id})
	if result[id] != 250 {
		t.Fatalf("expected 250 cents, got %d", result[id])
	}
}
