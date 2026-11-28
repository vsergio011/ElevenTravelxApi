package expenseledger

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestEconomicTotalUsesPerPersonPriceForIndividualDistribution(t *testing.T) {
	require.Equal(t, int64(3000), EconomicTotal(DistributionIndividual, 1000, 3))
	require.Equal(t, int64(1000), EconomicTotal(DistributionGroup, 1000, 3))
	require.Equal(t, int64(1000), EconomicTotal(DistributionSelected, 1000, 3))
}

func TestValidatePaymentsUsesEconomicTotal(t *testing.T) {
	payer := uuid.New()
	require.NoError(t, ValidatePayments(3000, []Payment{{UserID: payer, AmountCents: 3000}}))
	require.ErrorIs(t, ValidatePayments(3000, []Payment{{UserID: payer, AmountCents: 3001}}), ErrPaymentsExceedTotal)
}

func TestValidateSharesRejectsEmptyCustom(t *testing.T) {
	require.ErrorIs(t, ValidateShares(DistributionCustom, 1000, nil), ErrInvalidShares)
	require.ErrorIs(t, ValidateShares(DistributionCustom, 1000, []Share{}), ErrInvalidShares)
}

func TestAllocatePaidSharesProportionalAndExact(t *testing.T) {
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	shares := []Share{{UserID: a, Cents: 5000}, {UserID: b, Cents: 5000}}

	// Unpaid
	unpaid := AllocatePaidShares(shares, 0)
	require.Equal(t, int64(0), unpaid[0].Cents)
	require.Equal(t, int64(0), unpaid[1].Cents)

	// Partial payment (2000 out of 10000 -> 1000 each)
	partial := AllocatePaidShares(shares, 2000)
	require.Equal(t, int64(1000), partial[0].Cents)
	require.Equal(t, int64(1000), partial[1].Cents)

	// Odd remainder across 3 unequal shares
	three := []Share{{UserID: a, Cents: 1000}, {UserID: b, Cents: 1000}, {UserID: c, Cents: 1000}}
	odd := AllocatePaidShares(three, 100)
	var sum int64
	for _, s := range odd {
		sum += s.Cents
	}
	require.Equal(t, int64(100), sum)
}
