package transports

import (
	"context"
	"errors"
	"testing"

	"github.com/eleventravel/eleventravel-api/internal/plannings"
	"github.com/google/uuid"
)

func testInput() Input {
	return Input{Status: "draft", Type: "train", Origin: "Madrid", Destination: "Toledo", Currency: "EUR", CostDistribution: "group"}
}

func TestValidateInput(t *testing.T) {
	first, second := "2026-12-10", "2026-12-09"
	user := uuid.New()
	tests := []struct {
		name   string
		change func(*Input)
		valid  bool
	}{
		{"optional dates and payment", func(*Input) {}, true},
		{"unknown type", func(in *Input) { in.Type = "plane" }, false},
		{"negative cost", func(in *Input) { in.CostCents = -1 }, false},
		{"bad date", func(in *Input) { in.DepartureDate = &[]string{"2026-02-30"}[0] }, false},
		{"arrival before departure", func(in *Input) { in.DepartureDate = &first; in.ArrivalDate = &second }, false},
		{"selected without members", func(in *Input) { in.CostDistribution = "selected_participants" }, false},
		{"duplicate members", func(in *Input) { in.ParticipantUserIDs = []uuid.UUID{user, user} }, false},
		{"zero payment", func(in *Input) { in.CostCents = 100; in.Payments = &[]Payment{{UserID: user}} }, false},
		{"valid partial payment", func(in *Input) { in.CostCents = 100; in.Payments = &[]Payment{{UserID: user, AmountCents: 50}} }, true},
		{"valid external url", func(in *Input) { u := "https://www.renfe.com/es/es"; in.ExternalURL = &u }, true},
		{"empty external url", func(in *Input) { u := "   "; in.ExternalURL = &u }, true},
		{"invalid external url scheme", func(in *Input) { u := "javascript:alert(1)"; in.ExternalURL = &u }, false},
		{"invalid external url text", func(in *Input) { u := "not a url"; in.ExternalURL = &u }, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := testInput()
			tt.change(&in)
			err := validateInput(in)
			if (err == nil) != tt.valid {
				t.Fatalf("valid=%v, error=%v", tt.valid, err)
			}
		})
	}
}

type fakeMembership struct {
	members map[uuid.UUID]plannings.PlanningMember
}

func (m fakeMembership) GetMember(_ context.Context, _, user uuid.UUID) (plannings.PlanningMember, error) {
	member, ok := m.members[user]
	if !ok {
		return member, plannings.ErrMemberNotFound
	}
	return member, nil
}
func (m fakeMembership) ListMembers(context.Context, uuid.UUID) ([]plannings.PlanningMember, error) {
	items := make([]plannings.PlanningMember, 0, len(m.members))
	for _, member := range m.members {
		items = append(items, member)
	}
	return items, nil
}

type fakeRepository struct {
	item    Transport
	updated bool
}

func (r *fakeRepository) Create(_ context.Context, _, _ uuid.UUID, in Input) (Transport, error) {
	return Transport{Input: in}, nil
}
func (r *fakeRepository) Get(context.Context, uuid.UUID, uuid.UUID) (Transport, error) {
	return r.item, nil
}
func (r *fakeRepository) List(context.Context, uuid.UUID) ([]Transport, error) {
	return []Transport{r.item}, nil
}
func (r *fakeRepository) Update(_ context.Context, _, _ uuid.UUID, in Input) (Transport, error) {
	r.updated = true
	r.item.Input = in
	return r.item, nil
}
func (r *fakeRepository) Delete(context.Context, uuid.UUID, uuid.UUID) error {
	r.updated = true
	return nil
}

func TestEditPermissionsAndPaymentPreservation(t *testing.T) {
	creator, member, admin, outsider := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	planning, id := uuid.New(), uuid.New()
	payments := []Payment{{UserID: creator, AmountCents: 50}}
	input := testInput()
	input.CostCents = 100
	input.Payments = &payments
	repo := &fakeRepository{item: Transport{CreatedByUserID: creator, Input: input}}
	service := NewService(repo, fakeMembership{members: map[uuid.UUID]plannings.PlanningMember{
		creator: {UserID: creator, Role: plannings.RoleMember},
		member:  {UserID: member, Role: plannings.RoleMember},
		admin:   {UserID: admin, Role: plannings.RoleAdmin},
	}})
	changed := input
	changed.Payments = nil
	if _, err := service.Update(context.Background(), member, planning, id, changed); !errors.Is(err, ErrForbidden) {
		t.Fatalf("member error: %v", err)
	}
	if _, err := service.Get(context.Background(), outsider, planning, id); !errors.Is(err, ErrForbidden) {
		t.Fatalf("outsider error: %v", err)
	}
	if _, err := service.Update(context.Background(), creator, planning, id, changed); err != nil {
		t.Fatal(err)
	}
	if repo.item.Payments == nil || len(*repo.item.Payments) != 1 {
		t.Fatal("existing payment was not preserved")
	}
	if _, err := service.Confirm(context.Background(), admin, planning, id); err != nil {
		t.Fatal(err)
	}
	if repo.item.Status != "confirmed" {
		t.Fatal("confirmation failed")
	}
	if _, err := service.Confirm(context.Background(), creator, planning, id); err != nil {
		t.Fatal(err)
	}
	if repo.item.Status != "draft" {
		t.Fatal("return to draft failed")
	}
}

func TestServiceRejectsPaymentsAboveEconomicTotal(t *testing.T) {
	owner, member := uuid.New(), uuid.New()
	planning := uuid.New()
	input := testInput()
	input.CostCents = 100
	input.CostDistribution = "individual"
	input.Payments = &[]Payment{{UserID: owner, AmountCents: 201}}
	service := NewService(&fakeRepository{}, fakeMembership{members: map[uuid.UUID]plannings.PlanningMember{
		owner:  {UserID: owner, Role: plannings.RoleOwner},
		member: {UserID: member, Role: plannings.RoleMember},
	}})
	if _, err := service.Create(context.Background(), owner, planning, input); !errors.Is(err, ErrValidation) {
		t.Fatalf("expected economic overpayment validation, got %v", err)
	}
	input.Payments = &[]Payment{{UserID: owner, AmountCents: 200}}
	if _, err := service.Create(context.Background(), owner, planning, input); err != nil {
		t.Fatalf("expected payment matching individual total, got %v", err)
	}
}
