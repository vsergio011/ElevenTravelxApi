package transports

import (
	"context"
	"os"
	"testing"

	"github.com/eleventravel/eleventravel-api/internal/config"
	"github.com/eleventravel/eleventravel-api/internal/platform/db"
	"github.com/google/uuid"
)

func TestTransportLedgerIntegration(t *testing.T) {
	if os.Getenv("TRANSPORT_INTEGRATION_TEST") != "1" {
		t.Skip("set TRANSPORT_INTEGRATION_TEST=1 to run against the configured database")
	}
	ctx := context.Background()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.NewPostgresPool(ctx, cfg.DatabaseURL, cfg.DatabaseMaxConns)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	var planning, actor uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT planning_id, user_id FROM public.planning_members LIMIT 1`).Scan(&planning, &actor); err != nil {
		t.Fatal(err)
	}
	input := testInput()
	input.CostCents = 1001
	input.Payments = &[]Payment{{UserID: actor, AmountCents: 1001}}
	item, err := scan(tx.QueryRow(ctx, `INSERT INTO public.transports
		(planning_id, created_by_user_id, status, transport_type, origin, destination, currency, cost_distribution)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING `+columns,
		planning, actor, input.Status, input.Type, input.Origin, input.Destination, input.Currency, input.CostDistribution))
	if err != nil {
		t.Fatal(err)
	}
	if err := sync(ctx, tx, item, input); err != nil {
		t.Fatal(err)
	}
	repo := NewPostgresRepository(pool)
	loaded, err := repo.get(ctx, tx, planning, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.CostCents != 1001 || loaded.Payments == nil || len(*loaded.Payments) != 1 || (*loaded.Payments)[0].AmountCents != 1001 {
		t.Fatalf("transport expense or payment did not round-trip")
	}
	if _, err := tx.Exec(ctx, `SET CONSTRAINTS ALL IMMEDIATE`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `SET CONSTRAINTS ALL DEFERRED`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE public.transports SET status='confirmed' WHERE id=$1`, item.ID); err != nil {
		t.Fatal(err)
	}
	input.Payments = nil
	if err := sync(ctx, tx, item, input); err != nil {
		t.Fatal(err)
	}
	var status string
	var expenseCount, paymentCount int
	if err := tx.QueryRow(ctx, `SELECT t.status, (SELECT count(*) FROM public.expenses WHERE planning_item_id=t.planning_item_id),
		(SELECT count(*) FROM public.expense_payments p JOIN public.expenses e ON e.id=p.expense_id WHERE e.planning_item_id=t.planning_item_id)
		FROM public.transports t WHERE t.id=$1`, item.ID).Scan(&status, &expenseCount, &paymentCount); err != nil {
		t.Fatal(err)
	}
	if status != "confirmed" || expenseCount != 1 || paymentCount != 1 {
		t.Fatal("confirmation or repeated synchronization changed the expense or payment count")
	}
	if _, err := tx.Exec(ctx, `SET CONSTRAINTS ALL IMMEDIATE`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM public.transports WHERE id=$1`, item.ID); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM public.expenses WHERE planning_item_id=$1`, item.PlanningItemID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("deleting transport did not remove its linked expense")
	}
}
