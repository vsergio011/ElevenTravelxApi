package bookings

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/eleventravel/eleventravel-api/internal/plannings"
)

type fakeMembership struct {
	members map[uuid.UUID]plannings.PlanningMember
}

func (m *fakeMembership) GetMember(_ context.Context, planningID uuid.UUID, userID uuid.UUID) (plannings.PlanningMember, error) {
	member, ok := m.members[userID]
	if !ok || member.PlanningID != planningID {
		return plannings.PlanningMember{}, plannings.ErrMemberNotFound
	}
	return member, nil
}

type fakeRepository struct {
	bookings map[uuid.UUID]Booking

	createCalls  int
	updateCalls  int
	confirmCalls int
	deleteCalls  int
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{bookings: map[uuid.UUID]Booking{}}
}

func (r *fakeRepository) CreateBooking(_ context.Context, actorUserID uuid.UUID, planningID uuid.UUID, input CreateBookingInput) (Booking, error) {
	r.createCalls++
	booking := Booking{
		ID:              uuid.New(),
		PlanningID:      planningID,
		CreatedByUserID: actorUserID,
		Title:           input.Title,
		Type:            input.Type,
		Status:          input.Status,
		ExternalURL:     input.ExternalURL,
		OccursAt:        input.OccursAt,
		Location:        input.Location,
		Notes:           input.Notes,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	r.bookings[booking.ID] = booking
	return booking, nil
}

func (r *fakeRepository) GetBookingByID(_ context.Context, planningID uuid.UUID, bookingID uuid.UUID) (Booking, error) {
	booking, ok := r.bookings[bookingID]
	if !ok || booking.PlanningID != planningID {
		return Booking{}, ErrBookingNotFound
	}
	return booking, nil
}

func (r *fakeRepository) ListBookings(_ context.Context, planningID uuid.UUID, filters BookingFilters) (PageResult[Booking], error) {
	items := make([]Booking, 0)
	for _, booking := range r.bookings {
		if booking.PlanningID != planningID {
			continue
		}
		if filters.Status != nil && booking.Status != *filters.Status {
			continue
		}
		if filters.Type != nil && booking.Type != *filters.Type {
			continue
		}
		items = append(items, booking)
	}
	return PageResult[Booking]{Items: items, Page: filters.Page.Number, PageSize: filters.Page.Size, Total: len(items)}, nil
}

func (r *fakeRepository) UpdateBooking(_ context.Context, planningID uuid.UUID, bookingID uuid.UUID, input UpdateBookingInput) (Booking, error) {
	r.updateCalls++
	booking, ok := r.bookings[bookingID]
	if !ok || booking.PlanningID != planningID {
		return Booking{}, ErrBookingNotFound
	}
	if input.Title != nil {
		booking.Title = *input.Title
	}
	r.bookings[bookingID] = booking
	return booking, nil
}

func (r *fakeRepository) ConfirmBooking(_ context.Context, planningID uuid.UUID, bookingID uuid.UUID) (Booking, error) {
	r.confirmCalls++
	booking, ok := r.bookings[bookingID]
	if !ok || booking.PlanningID != planningID {
		return Booking{}, ErrBookingNotFound
	}
	booking.Status = StatusConfirmed
	r.bookings[bookingID] = booking
	return booking, nil
}

func (r *fakeRepository) DeleteBooking(_ context.Context, planningID uuid.UUID, bookingID uuid.UUID) error {
	r.deleteCalls++
	booking, ok := r.bookings[bookingID]
	if !ok || booking.PlanningID != planningID {
		return ErrBookingNotFound
	}
	delete(r.bookings, bookingID)
	return nil
}

func TestServiceCreateBookingRequiresMembership(t *testing.T) {
	t.Parallel()

	planningID := uuid.New()
	actorUserID := uuid.New()
	repo := newFakeRepository()
	membership := &fakeMembership{members: map[uuid.UUID]plannings.PlanningMember{}}
	service := NewService(repo, membership)

	_, err := service.CreateBooking(context.Background(), actorUserID, planningID, CreateBookingInput{Title: "Hotel Central", Type: TypeHotel, Status: StatusProposal})

	require.ErrorIs(t, err, ErrForbidden)
	require.Zero(t, repo.createCalls)
}

func TestServiceCreateBookingAllowsAnyMember(t *testing.T) {
	t.Parallel()

	planningID := uuid.New()
	actorUserID := uuid.New()
	repo := newFakeRepository()
	membership := &fakeMembership{members: map[uuid.UUID]plannings.PlanningMember{
		actorUserID: {PlanningID: planningID, UserID: actorUserID, Role: plannings.RoleMember},
	}}
	service := NewService(repo, membership)

	booking, err := service.CreateBooking(context.Background(), actorUserID, planningID, CreateBookingInput{Title: "Hotel Central", Type: TypeHotel, Status: StatusProposal})

	require.NoError(t, err)
	require.Equal(t, StatusProposal, booking.Status)
	require.Equal(t, 1, repo.createCalls)
}

func TestServiceUpdateBookingBlocksOtherMembers(t *testing.T) {
	t.Parallel()

	planningID := uuid.New()
	creatorUserID := uuid.New()
	otherMemberID := uuid.New()
	repo := newFakeRepository()
	booking, _ := repo.CreateBooking(context.Background(), creatorUserID, planningID, CreateBookingInput{Title: "Hotel Central", Type: TypeHotel, Status: StatusProposal})
	membership := &fakeMembership{members: map[uuid.UUID]plannings.PlanningMember{
		creatorUserID: {PlanningID: planningID, UserID: creatorUserID, Role: plannings.RoleMember},
		otherMemberID: {PlanningID: planningID, UserID: otherMemberID, Role: plannings.RoleMember},
	}}
	service := NewService(repo, membership)

	_, err := service.UpdateBooking(context.Background(), otherMemberID, planningID, booking.ID, UpdateBookingInput{Title: stringPointer("New title")})

	require.ErrorIs(t, err, ErrForbidden)
	require.Zero(t, repo.updateCalls)
}

func TestServiceUpdateBookingAllowsCreator(t *testing.T) {
	t.Parallel()

	planningID := uuid.New()
	creatorUserID := uuid.New()
	repo := newFakeRepository()
	booking, _ := repo.CreateBooking(context.Background(), creatorUserID, planningID, CreateBookingInput{Title: "Hotel Central", Type: TypeHotel, Status: StatusProposal})
	membership := &fakeMembership{members: map[uuid.UUID]plannings.PlanningMember{
		creatorUserID: {PlanningID: planningID, UserID: creatorUserID, Role: plannings.RoleMember},
	}}
	service := NewService(repo, membership)

	updated, err := service.UpdateBooking(context.Background(), creatorUserID, planningID, booking.ID, UpdateBookingInput{Title: stringPointer("New title")})

	require.NoError(t, err)
	require.Equal(t, "New title", updated.Title)
	require.Equal(t, 1, repo.updateCalls)
}

func TestServiceUpdateBookingAllowsAdmin(t *testing.T) {
	t.Parallel()

	planningID := uuid.New()
	creatorUserID := uuid.New()
	adminUserID := uuid.New()
	repo := newFakeRepository()
	booking, _ := repo.CreateBooking(context.Background(), creatorUserID, planningID, CreateBookingInput{Title: "Hotel Central", Type: TypeHotel, Status: StatusProposal})
	membership := &fakeMembership{members: map[uuid.UUID]plannings.PlanningMember{
		creatorUserID: {PlanningID: planningID, UserID: creatorUserID, Role: plannings.RoleMember},
		adminUserID:   {PlanningID: planningID, UserID: adminUserID, Role: plannings.RoleAdmin},
	}}
	service := NewService(repo, membership)

	_, err := service.UpdateBooking(context.Background(), adminUserID, planningID, booking.ID, UpdateBookingInput{Title: stringPointer("New title")})

	require.NoError(t, err)
	require.Equal(t, 1, repo.updateCalls)
}

func TestServiceConfirmBookingTransitionsStatus(t *testing.T) {
	t.Parallel()

	planningID := uuid.New()
	creatorUserID := uuid.New()
	repo := newFakeRepository()
	booking, _ := repo.CreateBooking(context.Background(), creatorUserID, planningID, CreateBookingInput{Title: "Hotel Central", Type: TypeHotel, Status: StatusProposal})
	membership := &fakeMembership{members: map[uuid.UUID]plannings.PlanningMember{
		creatorUserID: {PlanningID: planningID, UserID: creatorUserID, Role: plannings.RoleMember},
	}}
	service := NewService(repo, membership)

	confirmed, err := service.ConfirmBooking(context.Background(), creatorUserID, planningID, booking.ID)

	require.NoError(t, err)
	require.Equal(t, StatusConfirmed, confirmed.Status)
	require.Equal(t, 1, repo.confirmCalls)
}

func TestServiceConfirmBookingBlocksOtherMembers(t *testing.T) {
	t.Parallel()

	planningID := uuid.New()
	creatorUserID := uuid.New()
	otherMemberID := uuid.New()
	repo := newFakeRepository()
	booking, _ := repo.CreateBooking(context.Background(), creatorUserID, planningID, CreateBookingInput{Title: "Hotel Central", Type: TypeHotel, Status: StatusProposal})
	membership := &fakeMembership{members: map[uuid.UUID]plannings.PlanningMember{
		creatorUserID: {PlanningID: planningID, UserID: creatorUserID, Role: plannings.RoleMember},
		otherMemberID: {PlanningID: planningID, UserID: otherMemberID, Role: plannings.RoleMember},
	}}
	service := NewService(repo, membership)

	_, err := service.ConfirmBooking(context.Background(), otherMemberID, planningID, booking.ID)

	require.ErrorIs(t, err, ErrForbidden)
	require.Zero(t, repo.confirmCalls)
}

func TestServiceDeleteBookingAllowsAdminNotCreator(t *testing.T) {
	t.Parallel()

	planningID := uuid.New()
	creatorUserID := uuid.New()
	adminUserID := uuid.New()
	repo := newFakeRepository()
	booking, _ := repo.CreateBooking(context.Background(), creatorUserID, planningID, CreateBookingInput{Title: "Hotel Central", Type: TypeHotel, Status: StatusProposal})
	membership := &fakeMembership{members: map[uuid.UUID]plannings.PlanningMember{
		creatorUserID: {PlanningID: planningID, UserID: creatorUserID, Role: plannings.RoleMember},
		adminUserID:   {PlanningID: planningID, UserID: adminUserID, Role: plannings.RoleAdmin},
	}}
	service := NewService(repo, membership)

	err := service.DeleteBooking(context.Background(), adminUserID, planningID, booking.ID)

	require.NoError(t, err)
	require.Equal(t, 1, repo.deleteCalls)
}

func TestServiceListBookingsFiltersByStatusAndType(t *testing.T) {
	t.Parallel()

	planningID := uuid.New()
	actorUserID := uuid.New()
	repo := newFakeRepository()
	_, _ = repo.CreateBooking(context.Background(), actorUserID, planningID, CreateBookingInput{Title: "Hotel Central", Type: TypeHotel, Status: StatusProposal})
	confirmedBooking, _ := repo.CreateBooking(context.Background(), actorUserID, planningID, CreateBookingInput{Title: "Restaurant X", Type: TypeRestaurant, Status: StatusProposal})
	confirmedBooking.Status = StatusConfirmed
	repo.bookings[confirmedBooking.ID] = confirmedBooking
	membership := &fakeMembership{members: map[uuid.UUID]plannings.PlanningMember{
		actorUserID: {PlanningID: planningID, UserID: actorUserID, Role: plannings.RoleMember},
	}}
	service := NewService(repo, membership)

	status := StatusConfirmed
	result, err := service.ListBookings(context.Background(), actorUserID, planningID, BookingFilters{Page: Page{Number: 1, Size: 20}, Status: &status})

	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	require.Equal(t, TypeRestaurant, result.Items[0].Type)
}

func stringPointer(value string) *string {
	return &value
}
