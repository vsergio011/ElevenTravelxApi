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
