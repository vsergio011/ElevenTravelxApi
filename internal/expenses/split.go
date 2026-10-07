package expenses

import (
	"sort"

	"github.com/google/uuid"
)

const (
	distributionIndividual = "individual"
)

// computeShares splits cents equally among the eligible users in integer cents.
// Remainders go to the first users in UUID order, matching bookings.AllocateCost.
// "individual" means every eligible user owes the full amount.
func computeShares(distribution string, amountCents int64, allMembers []uuid.UUID, selected []uuid.UUID) []Share {
	eligible := allMembers
	if distribution == DistributionSelectedParticipants {
		eligible = selected
	}
	sorted := append([]uuid.UUID(nil), eligible...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].String() < sorted[j].String() })
	shares := make([]Share, 0, len(sorted))
	if len(sorted) == 0 {
		return shares
	}
	if distribution == distributionIndividual {
		for _, id := range sorted {
			shares = append(shares, Share{UserID: id, Cents: amountCents})
		}
		return shares
	}
	count := int64(len(sorted))
	base, remainder := amountCents/count, amountCents%count
	for i, id := range sorted {
		cents := base
		if int64(i) < remainder {
			cents++
		}
		shares = append(shares, Share{UserID: id, Cents: cents})
	}
	return shares
}

// suggestSettlements greedily matches debtors with creditors so the fewest transfers settle the group.
func suggestSettlements(balances map[uuid.UUID]int64) []Settlement {
	type entry struct {
		id    uuid.UUID
		cents int64
	}
	debtors, creditors := []entry{}, []entry{}
	for id, cents := range balances {
		if cents < 0 {
			debtors = append(debtors, entry{id, -cents})
		} else if cents > 0 {
			creditors = append(creditors, entry{id, cents})
		}
	}
	order := func(list []entry) {
		sort.Slice(list, func(i, j int) bool {
			if list[i].cents != list[j].cents {
				return list[i].cents > list[j].cents
			}
			return list[i].id.String() < list[j].id.String()
		})
	}
	order(debtors)
	order(creditors)
	result := []Settlement{}
	i, j := 0, 0
	for i < len(debtors) && j < len(creditors) {
		amount := min(debtors[i].cents, creditors[j].cents)
		result = append(result, Settlement{FromUserID: debtors[i].id, ToUserID: creditors[j].id, AmountCents: amount})
		debtors[i].cents -= amount
		creditors[j].cents -= amount
		if debtors[i].cents == 0 {
			i++
		}
		if creditors[j].cents == 0 {
			j++
		}
	}
	return result
}
