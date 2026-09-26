package bookings

import (
	"github.com/google/uuid"
	"sort"
)

// ParticipantCost is the domain result consumed by API and UI budget views.
type ParticipantCost struct {
	UserID uuid.UUID
	Cents  int64
}

// AllocateCost is the single source of truth for booking cost distribution.
// Amounts are integer cents. Remainders are assigned deterministically by UUID order.
func AllocateCost(booking Booking, participantIDs []uuid.UUID) []ParticipantCost {
	result := make([]ParticipantCost, 0, len(participantIDs))
	if booking.Status != StatusConfirmed || len(participantIDs) == 0 {
		return result
	}
	eligible := participantIDs
	if booking.CostDistribution == DistributionSelectedParticipants {
		eligible = append([]uuid.UUID(nil), booking.ParticipantUserIDs...)
	}
	sort.Slice(eligible, func(i, j int) bool { return eligible[i].String() < eligible[j].String() })
	if booking.CostDistribution == DistributionIndividual {
		for _, id := range eligible {
			result = append(result, ParticipantCost{UserID: id, Cents: booking.CostCents})
		}
		return result
	}
	if len(eligible) == 0 {
		return result
	}
	base, remainder := booking.CostCents/int64(len(eligible)), booking.CostCents%int64(len(eligible))
	for i, id := range eligible {
		cents := base
		if int64(i) < remainder {
			cents++
		}
		result = append(result, ParticipantCost{UserID: id, Cents: cents})
	}
	return result
}

func IndividualBudget(bookings []Booking, participantIDs []uuid.UUID) map[uuid.UUID]int64 {
	result := make(map[uuid.UUID]int64, len(participantIDs))
	for _, id := range participantIDs {
		result[id] = 0
	}
	for _, booking := range bookings {
		for _, allocation := range AllocateCost(booking, participantIDs) {
			result[allocation.UserID] += allocation.Cents
		}
	}
	return result
}
