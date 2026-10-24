package expenses

import (
	"sort"

	"github.com/google/uuid"

	"github.com/eleventravel/eleventravel-api/internal/expenseledger"
)

// computeShares delegates to the shared ledger so every module splits cents the same way.
func computeShares(distribution string, amountCents int64, allMembers []uuid.UUID, selected []uuid.UUID) []Share {
	return expenseledger.ComputeShares(distribution, amountCents, allMembers, selected, nil)
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
