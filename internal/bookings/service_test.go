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
	members           map[uuid.UUID]plannings.PlanningMember
	routeStopCreators map[uuid.UUID]uuid.UUID
}

func (m *fakeMembership) GetMember(_ context.Context, planningID uuid.UUID, userID uuid.UUID) (plannings.PlanningMember, error) {
	member, ok := m.members[userID]
	if !ok || member.PlanningID != planningID {
		return plannings.PlanningMember{}, plannings.ErrMemberNotFound
	}
	return member, nil
}

func (m *fakeMembership) GetPlanningRouteStopCreator(_ context.Context, _ uuid.UUID, routeID uuid.UUID) (uuid.UUID, error) {
	creatorUserID, ok := m.routeStopCreators[routeID]
	if !ok {
		return uuid.Nil, plannings.ErrPlanningNotFound
	}
	return creatorUserID, nil
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
		RouteStopID:     input.RouteStopID,
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
	if input.RouteStopIDSet {
		booking.RouteStopID = input.RouteStopID
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

func TestServiceCreateBookingAllowsPointCreatorToAssociateBooking(t *testing.T) {
	t.Parallel()

	planningID := uuid.New()
	actorUserID := uuid.New()
	routeStopID := uuid.New()
	repo := newFakeRepository()
	membership := &fakeMembership{
		members: map[uuid.UUID]plannings.PlanningMember{
			actorUserID: {PlanningID: planningID, UserID: actorUserID, Role: plannings.RoleMember},
		},
		routeStopCreators: map[uuid.UUID]uuid.UUID{routeStopID: actorUserID},
	}
	service := NewService(repo, membership)

	booking, err := service.CreateBooking(context.Background(), actorUserID, planningID, CreateBookingInput{
		Title: "Hotel Central", Type: TypeHotel, Status: StatusProposal, RouteStopID: &routeStopID,
	})

	require.NoError(t, err)
	require.Equal(t, &routeStopID, booking.RouteStopID)
	require.Equal(t, 1, repo.createCalls)
}

func TestServiceCreateBookingBlocksAssociationToAnotherMembersPoint(t *testing.T) {
	t.Parallel()

	planningID := uuid.New()
	actorUserID := uuid.New()
	routeCreatorID := uuid.New()
	routeStopID := uuid.New()
	repo := newFakeRepository()
	membership := &fakeMembership{
		members: map[uuid.UUID]plannings.PlanningMember{
			actorUserID: {PlanningID: planningID, UserID: actorUserID, Role: plannings.RoleMember},
		},
		routeStopCreators: map[uuid.UUID]uuid.UUID{routeStopID: routeCreatorID},
	}
	service := NewService(repo, membership)

	_, err := service.CreateBooking(context.Background(), actorUserID, planningID, CreateBookingInput{
		Title: "Hotel Central", Type: TypeHotel, Status: StatusProposal, RouteStopID: &routeStopID,
	})

	require.ErrorIs(t, err, ErrForbidden)
	require.Zero(t, repo.createCalls)
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

func TestServiceUpdateBookingAllowsRouteStopCreatorToAssociateBooking(t *testing.T) {
	t.Parallel()

	planningID := uuid.New()
	bookingCreatorID := uuid.New()
	stopCreatorID := uuid.New()
	routeStopID := uuid.New()
	repo := newFakeRepository()
	booking, _ := repo.CreateBooking(context.Background(), bookingCreatorID, planningID, CreateBookingInput{Title: "Hotel Central", Type: TypeHotel, Status: StatusProposal})
	membership := &fakeMembership{
		members: map[uuid.UUID]plannings.PlanningMember{
			bookingCreatorID: {PlanningID: planningID, UserID: bookingCreatorID, Role: plannings.RoleMember},
			stopCreatorID:    {PlanningID: planningID, UserID: stopCreatorID, Role: plannings.RoleMember},
		},
		routeStopCreators: map[uuid.UUID]uuid.UUID{routeStopID: stopCreatorID},
	}
	service := NewService(repo, membership)

	updated, err := service.UpdateBooking(context.Background(), stopCreatorID, planningID, booking.ID, UpdateBookingInput{
		RouteStopID:    &routeStopID,
		RouteStopIDSet: true,
	})

	require.NoError(t, err)
	require.Equal(t, &routeStopID, updated.RouteStopID)
	require.Equal(t, 1, repo.updateCalls)
}

func TestServiceUpdateBookingAllowsRouteStopCreatorToDisassociateBooking(t *testing.T) {
	t.Parallel()

	planningID := uuid.New()
	bookingCreatorID := uuid.New()
	stopCreatorID := uuid.New()
	routeStopID := uuid.New()
	repo := newFakeRepository()
	booking, _ := repo.CreateBooking(context.Background(), bookingCreatorID, planningID, CreateBookingInput{Title: "Hotel Central", Type: TypeHotel, Status: StatusProposal})
	booking.RouteStopID = &routeStopID
	repo.bookings[booking.ID] = booking
	membership := &fakeMembership{
		members: map[uuid.UUID]plannings.PlanningMember{
			bookingCreatorID: {PlanningID: planningID, UserID: bookingCreatorID, Role: plannings.RoleMember},
			stopCreatorID:    {PlanningID: planningID, UserID: stopCreatorID, Role: plannings.RoleMember},
		},
		routeStopCreators: map[uuid.UUID]uuid.UUID{routeStopID: stopCreatorID},
	}
	service := NewService(repo, membership)

	updated, err := service.UpdateBooking(context.Background(), stopCreatorID, planningID, booking.ID, UpdateBookingInput{RouteStopIDSet: true})

	require.NoError(t, err)
	require.Nil(t, updated.RouteStopID)
	require.Equal(t, 1, repo.updateCalls)
}

func TestServiceUpdateBookingBlocksUnrelatedMemberFromAssociation(t *testing.T) {
	t.Parallel()

	planningID := uuid.New()
	bookingCreatorID := uuid.New()
	otherMemberID := uuid.New()
	stopCreatorID := uuid.New()
	routeStopID := uuid.New()
	repo := newFakeRepository()
	booking, _ := repo.CreateBooking(context.Background(), bookingCreatorID, planningID, CreateBookingInput{Title: "Hotel Central", Type: TypeHotel, Status: StatusProposal})
	membership := &fakeMembership{
		members: map[uuid.UUID]plannings.PlanningMember{
			bookingCreatorID: {PlanningID: planningID, UserID: bookingCreatorID, Role: plannings.RoleMember},
			otherMemberID:    {PlanningID: planningID, UserID: otherMemberID, Role: plannings.RoleMember},
		},
		routeStopCreators: map[uuid.UUID]uuid.UUID{routeStopID: stopCreatorID},
	}
	service := NewService(repo, membership)

	_, err := service.UpdateBooking(context.Background(), otherMemberID, planningID, booking.ID, UpdateBookingInput{
		RouteStopID:    &routeStopID,
		RouteStopIDSet: true,
	})

	require.ErrorIs(t, err, ErrForbidden)
	require.Zero(t, repo.updateCalls)
}

func TestServiceUpdateBookingCannotMoveBookingFromAnotherMembersStop(t *testing.T) {
	t.Parallel()

	planningID := uuid.New()
	bookingCreatorID := uuid.New()
	stopCreatorID := uuid.New()
	otherStopCreatorID := uuid.New()
	currentStopID := uuid.New()
	nextStopID := uuid.New()
	repo := newFakeRepository()
	booking, _ := repo.CreateBooking(context.Background(), bookingCreatorID, planningID, CreateBookingInput{Title: "Hotel Central", Type: TypeHotel, Status: StatusProposal})
	booking.RouteStopID = &currentStopID
	repo.bookings[booking.ID] = booking
	membership := &fakeMembership{
		members: map[uuid.UUID]plannings.PlanningMember{
			bookingCreatorID: {PlanningID: planningID, UserID: bookingCreatorID, Role: plannings.RoleMember},
			stopCreatorID:    {PlanningID: planningID, UserID: stopCreatorID, Role: plannings.RoleMember},
		},
		routeStopCreators: map[uuid.UUID]uuid.UUID{
			currentStopID: otherStopCreatorID,
			nextStopID:    stopCreatorID,
		},
	}
	service := NewService(repo, membership)

	_, err := service.UpdateBooking(context.Background(), stopCreatorID, planningID, booking.ID, UpdateBookingInput{
		RouteStopID:    &nextStopID,
		RouteStopIDSet: true,
	})

	require.ErrorIs(t, err, ErrForbidden)
	require.Zero(t, repo.updateCalls)
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
