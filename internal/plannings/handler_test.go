package plannings

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/eleventravel/eleventravel-api/internal/http/middleware"
	"github.com/eleventravel/eleventravel-api/internal/platform/auth/supabasejwt"
)

func TestPlanningRoutesRejectUnauthenticatedRequests(t *testing.T) {
	t.Parallel()

	planningID := uuid.New()
	router := newPlanningTestRouter(&fakePlanningService{}, fakeTokenValidator{})
	request := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v1/plannings/%s", planningID), nil)
	responseRecorder := httptest.NewRecorder()

	router.ServeHTTP(responseRecorder, request)

	require.Equal(t, http.StatusUnauthorized, responseRecorder.Code)
	require.Contains(t, responseRecorder.Body.String(), "unauthorized")
}

func TestPlanningRoutesAllowAuthenticatedMembersToRead(t *testing.T) {
	t.Parallel()

	planningID := uuid.New()
	actorUserID := uuid.New()
	planningGroupID := uuid.New()
	router := newPlanningTestRouter(&fakePlanningService{
		getPlanningFunc: func(_ context.Context, userID uuid.UUID, requestedPlanningID uuid.UUID) (Planning, error) {
			require.Equal(t, actorUserID, userID)
			require.Equal(t, planningID, requestedPlanningID)
			return Planning{ID: planningID, GroupID: &planningGroupID, OwnerUserID: actorUserID, Name: "Summer Trip", Status: StatusDraft, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}, nil
		},
	}, fakeTokenValidator{claims: supabasejwt.Claims{RegisteredClaims: registeredClaims(actorUserID)}})

	request := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v1/plannings/%s", planningID), nil)
	request.Header.Set("Authorization", "Bearer valid-token")
	responseRecorder := httptest.NewRecorder()

	router.ServeHTTP(responseRecorder, request)

	require.Equal(t, http.StatusOK, responseRecorder.Code)
	require.Contains(t, responseRecorder.Body.String(), planningID.String())
}

func TestPlanningRoutesReturnForbiddenForMemberUpdate(t *testing.T) {
	t.Parallel()

	planningID := uuid.New()
	actorUserID := uuid.New()
	router := newPlanningTestRouter(&fakePlanningService{
		updatePlanningFunc: func(_ context.Context, userID uuid.UUID, requestedPlanningID uuid.UUID, input UpdatePlanningInput) (Planning, error) {
			require.Equal(t, actorUserID, userID)
			require.Equal(t, planningID, requestedPlanningID)
			require.NotNil(t, input.Name)
			return Planning{}, ErrForbidden
		},
	}, fakeTokenValidator{claims: supabasejwt.Claims{RegisteredClaims: registeredClaims(actorUserID)}})

	request := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/v1/plannings/%s", planningID), strings.NewReader(`{"name":"Blocked"}`))
	request.Header.Set("Authorization", "Bearer valid-token")
	request.Header.Set("Content-Type", "application/json")
	responseRecorder := httptest.NewRecorder()

	router.ServeHTTP(responseRecorder, request)

	require.Equal(t, http.StatusForbidden, responseRecorder.Code)
	require.Contains(t, responseRecorder.Body.String(), "forbidden")
}

func TestPlanningRoutesDeletePlanningReturnsNoContent(t *testing.T) {
	t.Parallel()

	planningID := uuid.New()
	actorUserID := uuid.New()
	router := newPlanningTestRouter(&fakePlanningService{
		deletePlanningFunc: func(_ context.Context, userID uuid.UUID, requestedPlanningID uuid.UUID) error {
			require.Equal(t, actorUserID, userID)
			require.Equal(t, planningID, requestedPlanningID)
			return nil
		},
	}, fakeTokenValidator{claims: supabasejwt.Claims{RegisteredClaims: registeredClaims(actorUserID)}})

	request := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/v1/plannings/%s", planningID), nil)
	request.Header.Set("Authorization", "Bearer valid-token")
	responseRecorder := httptest.NewRecorder()

	router.ServeHTTP(responseRecorder, request)

	require.Equal(t, http.StatusNoContent, responseRecorder.Code)
	require.Empty(t, responseRecorder.Body.String())
}

func TestPlanningRouteItineraryReturnsItineraryPayload(t *testing.T) {
	t.Parallel()

	planningID := uuid.New()
	actorUserID := uuid.New()
	router := newPlanningTestRouter(&fakePlanningService{
		getPlanningItineraryFunc: func(_ context.Context, userID uuid.UUID, requestedPlanningID uuid.UUID) (PlanningItinerary, error) {
			require.Equal(t, actorUserID, userID)
			require.Equal(t, planningID, requestedPlanningID)
			return PlanningItinerary{
				PlanningID:   planningID,
				PlanningName: "Amalfi",
				Days: []PlanningItineraryDay{{
					ID:        uuid.New(),
					Label:     "Día 1",
					DateLabel: "01 Ago",
					Stops: []PlanningItineraryStop{{
						ID:      uuid.New(),
						Title:   "Piazza",
						Address: "Via Roma",
						Coordinates: PlanningItineraryCoordinates{
							Latitude:  40.1,
							Longitude: 14.2,
						},
					}},
				}},
			}, nil
		},
	}, fakeTokenValidator{claims: supabasejwt.Claims{RegisteredClaims: registeredClaims(actorUserID)}})

	request := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v1/plannings/%s/routes", planningID), nil)
	request.Header.Set("Authorization", "Bearer valid-token")
	responseRecorder := httptest.NewRecorder()

	router.ServeHTTP(responseRecorder, request)

	require.Equal(t, http.StatusOK, responseRecorder.Code)
	require.Contains(t, responseRecorder.Body.String(), "Amalfi")
	require.Contains(t, responseRecorder.Body.String(), "Piazza")
}

func TestPlanningRouteCreateStopReturnsCreated(t *testing.T) {
	t.Parallel()

	planningID := uuid.New()
	dayID := uuid.New()
	actorUserID := uuid.New()
	createdStopID := uuid.New()

	router := newPlanningTestRouter(&fakePlanningService{
		createPlanningRouteStopFunc: func(_ context.Context, userID uuid.UUID, requestedPlanningID uuid.UUID, input CreatePlanningRouteStopInput) (PlanningItineraryStop, error) {
			require.Equal(t, actorUserID, userID)
			require.Equal(t, planningID, requestedPlanningID)
			require.Equal(t, dayID, input.DayID)
			require.Equal(t, "Nuevo punto", input.Title)
			return PlanningItineraryStop{ID: createdStopID, Title: input.Title}, nil
		},
	}, fakeTokenValidator{claims: supabasejwt.Claims{RegisteredClaims: registeredClaims(actorUserID)}})

	requestBody := fmt.Sprintf(`{"day_id":"%s","title":"Nuevo punto","description":"","address":"","time_label":"","duration_minutes":30,"notes":"","cost_estimate":0,"currency":"EUR","external_url":"","status":"pending","category":"landmark","is_optional":false,"coordinates":{"latitude":40.1,"longitude":14.2}}`, dayID)
	request := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/plannings/%s/routes", planningID), strings.NewReader(requestBody))
	request.Header.Set("Authorization", "Bearer valid-token")
	request.Header.Set("Content-Type", "application/json")
	responseRecorder := httptest.NewRecorder()

	router.ServeHTTP(responseRecorder, request)

	require.Equal(t, http.StatusCreated, responseRecorder.Code)
	require.Contains(t, responseRecorder.Body.String(), createdStopID.String())
	require.Contains(t, responseRecorder.Body.String(), "Nuevo punto")
}

func TestPlanningRouteCreateStopReturnsValidationError(t *testing.T) {
	t.Parallel()

	planningID := uuid.New()
	actorUserID := uuid.New()
	router := newPlanningTestRouter(&fakePlanningService{}, fakeTokenValidator{claims: supabasejwt.Claims{RegisteredClaims: registeredClaims(actorUserID)}})

	request := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/plannings/%s/routes", planningID), strings.NewReader(`{"day_id":"bad-id","title":"","duration_minutes":-1,"cost_estimate":-1}`))
	request.Header.Set("Authorization", "Bearer valid-token")
	request.Header.Set("Content-Type", "application/json")
	responseRecorder := httptest.NewRecorder()

	router.ServeHTTP(responseRecorder, request)

	require.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	require.Contains(t, responseRecorder.Body.String(), "validation_error")
}

func TestPlanningRouteUpdateStopReturnsUpdatedStop(t *testing.T) {
	t.Parallel()

	planningID := uuid.New()
	routeID := uuid.New()
	actorUserID := uuid.New()

	router := newPlanningTestRouter(&fakePlanningService{
		updatePlanningRouteStopFunc: func(_ context.Context, userID uuid.UUID, requestedPlanningID uuid.UUID, requestedRouteID uuid.UUID, input UpdatePlanningRouteStopInput) (PlanningItineraryStop, error) {
			require.Equal(t, actorUserID, userID)
			require.Equal(t, planningID, requestedPlanningID)
			require.Equal(t, routeID, requestedRouteID)
			require.NotNil(t, input.Title)
			require.Equal(t, "Editado", *input.Title)
			return PlanningItineraryStop{ID: routeID, Title: *input.Title}, nil
		},
	}, fakeTokenValidator{claims: supabasejwt.Claims{RegisteredClaims: registeredClaims(actorUserID)}})

	request := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/v1/plannings/%s/routes/%s", planningID, routeID), strings.NewReader(`{"title":"Editado"}`))
	request.Header.Set("Authorization", "Bearer valid-token")
	request.Header.Set("Content-Type", "application/json")
	responseRecorder := httptest.NewRecorder()

	router.ServeHTTP(responseRecorder, request)

	require.Equal(t, http.StatusOK, responseRecorder.Code)
	require.Contains(t, responseRecorder.Body.String(), routeID.String())
	require.Contains(t, responseRecorder.Body.String(), "Editado")
}

func TestPlanningRouteDeleteStopReturnsDeletedStatus(t *testing.T) {
	t.Parallel()

	planningID := uuid.New()
	routeID := uuid.New()
	actorUserID := uuid.New()

	router := newPlanningTestRouter(&fakePlanningService{
		deletePlanningRouteStopFunc: func(_ context.Context, userID uuid.UUID, requestedPlanningID uuid.UUID, requestedRouteID uuid.UUID) error {
			require.Equal(t, actorUserID, userID)
			require.Equal(t, planningID, requestedPlanningID)
			require.Equal(t, routeID, requestedRouteID)
			return nil
		},
	}, fakeTokenValidator{claims: supabasejwt.Claims{RegisteredClaims: registeredClaims(actorUserID)}})

	request := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/v1/plannings/%s/routes/%s", planningID, routeID), nil)
	request.Header.Set("Authorization", "Bearer valid-token")
	responseRecorder := httptest.NewRecorder()

	router.ServeHTTP(responseRecorder, request)

	require.Equal(t, http.StatusOK, responseRecorder.Code)
	require.Contains(t, responseRecorder.Body.String(), "deleted")
}

func TestPlanningRouteReorderReturnsReorderedStatus(t *testing.T) {
	t.Parallel()

	planningID := uuid.New()
	dayID := uuid.New()
	stopIDOne := uuid.New()
	stopIDTwo := uuid.New()
	actorUserID := uuid.New()

	router := newPlanningTestRouter(&fakePlanningService{
		reorderPlanningRouteStopsFunc: func(_ context.Context, userID uuid.UUID, requestedPlanningID uuid.UUID, input ReorderPlanningRouteStopsInput) error {
			require.Equal(t, actorUserID, userID)
			require.Equal(t, planningID, requestedPlanningID)
			require.Equal(t, dayID, input.DayID)
			require.Equal(t, []uuid.UUID{stopIDOne, stopIDTwo}, input.StopIDs)
			return nil
		},
	}, fakeTokenValidator{claims: supabasejwt.Claims{RegisteredClaims: registeredClaims(actorUserID)}})

	requestBody := fmt.Sprintf(`{"day_id":"%s","stop_ids":["%s","%s"]}`, dayID, stopIDOne, stopIDTwo)
	request := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/plannings/%s/routes/reorder", planningID), strings.NewReader(requestBody))
	request.Header.Set("Authorization", "Bearer valid-token")
	request.Header.Set("Content-Type", "application/json")
	responseRecorder := httptest.NewRecorder()

	router.ServeHTTP(responseRecorder, request)

	require.Equal(t, http.StatusOK, responseRecorder.Code)
	require.Contains(t, responseRecorder.Body.String(), "reordered")
}

func TestPlanningRouteReorderReturnsValidationErrorForDuplicatedStopIDs(t *testing.T) {
	t.Parallel()

	planningID := uuid.New()
	dayID := uuid.New()
	stopID := uuid.New()
	actorUserID := uuid.New()
	router := newPlanningTestRouter(&fakePlanningService{}, fakeTokenValidator{claims: supabasejwt.Claims{RegisteredClaims: registeredClaims(actorUserID)}})

	requestBody := fmt.Sprintf(`{"day_id":"%s","stop_ids":["%s","%s"]}`, dayID, stopID, stopID)
	request := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/plannings/%s/routes/reorder", planningID), strings.NewReader(requestBody))
	request.Header.Set("Authorization", "Bearer valid-token")
	request.Header.Set("Content-Type", "application/json")
	responseRecorder := httptest.NewRecorder()

	router.ServeHTTP(responseRecorder, request)

	require.Equal(t, http.StatusBadRequest, responseRecorder.Code)
	require.Contains(t, responseRecorder.Body.String(), "validation_error")
}

func newPlanningTestRouter(service PlanningService, validator supabasejwt.TokenValidator) http.Handler {
	handler := NewHandler(service)
	router := chi.NewRouter()
	router.Route("/v1", func(api chi.Router) {
		api.Use(middleware.Authenticate(validator))
		handler.RegisterRoutes(api)
	})
	return router
}

type fakeTokenValidator struct {
	claims supabasejwt.Claims
	err    error
}

func (v fakeTokenValidator) ValidateToken(_ context.Context, _ string) (supabasejwt.Claims, error) {
	return v.claims, v.err
}

type fakePlanningService struct {
	createPlanningFunc            func(ctx context.Context, actorUserID uuid.UUID, input CreatePlanningInput) (Planning, error)
	getPlanningFunc               func(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID) (Planning, error)
	getSummaryFunc                func(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID) (PlanningSummary, error)
	getDashboardActivityFunc      func(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID) (PlanningDashboardActivity, error)
	listPlanningsFunc             func(ctx context.Context, actorUserID uuid.UUID, filters PlanningFilters) (PageResult[Planning], error)
	updatePlanningFunc            func(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, input UpdatePlanningInput) (Planning, error)
	deletePlanningFunc            func(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID) error
	archivePlanningFunc           func(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID) (Planning, error)
	unarchivePlanningFunc         func(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID) (Planning, error)
	listMembersFunc               func(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID) ([]PlanningMember, error)
	addMemberFunc                 func(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, input AddPlanningMemberInput) (PlanningMember, error)
	updateMemberRoleFunc          func(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, targetUserID uuid.UUID, input UpdatePlanningMemberRoleInput) (PlanningMember, error)
	removeMemberFunc              func(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, targetUserID uuid.UUID) error
	listActivityFunc              func(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, page Page) (PageResult[PlanningActivity], error)
	getPlanningItineraryFunc      func(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID) (PlanningItinerary, error)
	createPlanningRouteDayFunc    func(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, input CreatePlanningRouteDayInput) (PlanningItineraryDay, error)
	deletePlanningRouteDayFunc    func(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, dayID uuid.UUID) error
	getPlanningRouteByIDFunc      func(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, routeID uuid.UUID) (PlanningItineraryStop, error)
	createPlanningRouteStopFunc   func(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, input CreatePlanningRouteStopInput) (PlanningItineraryStop, error)
	updatePlanningRouteStopFunc   func(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, routeID uuid.UUID, input UpdatePlanningRouteStopInput) (PlanningItineraryStop, error)
	deletePlanningRouteStopFunc   func(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, routeID uuid.UUID) error
	reorderPlanningRouteStopsFunc func(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, input ReorderPlanningRouteStopsInput) error
}

func (s *fakePlanningService) CreatePlanning(ctx context.Context, actorUserID uuid.UUID, input CreatePlanningInput) (Planning, error) {
	if s.createPlanningFunc != nil {
		return s.createPlanningFunc(ctx, actorUserID, input)
	}
	return Planning{}, nil
}

func (s *fakePlanningService) GetPlanning(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID) (Planning, error) {
	if s.getPlanningFunc != nil {
		return s.getPlanningFunc(ctx, actorUserID, planningID)
	}
	return Planning{}, nil
}

func (s *fakePlanningService) GetSummary(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID) (PlanningSummary, error) {
	if s.getSummaryFunc != nil {
		return s.getSummaryFunc(ctx, actorUserID, planningID)
	}
	return PlanningSummary{}, nil
}

func (s *fakePlanningService) GetDashboardActivity(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID) (PlanningDashboardActivity, error) {
	if s.getDashboardActivityFunc != nil {
		return s.getDashboardActivityFunc(ctx, actorUserID, planningID)
	}
	return PlanningDashboardActivity{}, nil
}

func (s *fakePlanningService) ListPlannings(ctx context.Context, actorUserID uuid.UUID, filters PlanningFilters) (PageResult[Planning], error) {
	if s.listPlanningsFunc != nil {
		return s.listPlanningsFunc(ctx, actorUserID, filters)
	}
	return PageResult[Planning]{Items: []Planning{}, Page: 1, PageSize: 20}, nil
}

func (s *fakePlanningService) UpdatePlanning(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, input UpdatePlanningInput) (Planning, error) {
	if s.updatePlanningFunc != nil {
		return s.updatePlanningFunc(ctx, actorUserID, planningID, input)
	}
	return Planning{}, nil
}

func (s *fakePlanningService) DeletePlanning(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID) error {
	if s.deletePlanningFunc != nil {
		return s.deletePlanningFunc(ctx, actorUserID, planningID)
	}
	return nil
}

func (s *fakePlanningService) ArchivePlanning(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID) (Planning, error) {
	if s.archivePlanningFunc != nil {
		return s.archivePlanningFunc(ctx, actorUserID, planningID)
	}
	return Planning{}, nil
}

func (s *fakePlanningService) UnarchivePlanning(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID) (Planning, error) {
	if s.unarchivePlanningFunc != nil {
		return s.unarchivePlanningFunc(ctx, actorUserID, planningID)
	}
	return Planning{}, nil
}

func (s *fakePlanningService) ListMembers(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID) ([]PlanningMember, error) {
	if s.listMembersFunc != nil {
		return s.listMembersFunc(ctx, actorUserID, planningID)
	}
	return []PlanningMember{}, nil
}

func (s *fakePlanningService) AddMember(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, input AddPlanningMemberInput) (PlanningMember, error) {
	if s.addMemberFunc != nil {
		return s.addMemberFunc(ctx, actorUserID, planningID, input)
	}
	return PlanningMember{}, nil
}

func (s *fakePlanningService) UpdateMemberRole(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, targetUserID uuid.UUID, input UpdatePlanningMemberRoleInput) (PlanningMember, error) {
	if s.updateMemberRoleFunc != nil {
		return s.updateMemberRoleFunc(ctx, actorUserID, planningID, targetUserID, input)
	}
	return PlanningMember{}, nil
}

func (s *fakePlanningService) RemoveMember(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, targetUserID uuid.UUID) error {
	if s.removeMemberFunc != nil {
		return s.removeMemberFunc(ctx, actorUserID, planningID, targetUserID)
	}
	return nil
}

func (s *fakePlanningService) ListActivity(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, page Page) (PageResult[PlanningActivity], error) {
	if s.listActivityFunc != nil {
		return s.listActivityFunc(ctx, actorUserID, planningID, page)
	}
	return PageResult[PlanningActivity]{Items: []PlanningActivity{}, Page: 1, PageSize: 20}, nil
}

func (s *fakePlanningService) GetPlanningItinerary(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID) (PlanningItinerary, error) {
	if s.getPlanningItineraryFunc != nil {
		return s.getPlanningItineraryFunc(ctx, actorUserID, planningID)
	}
	return PlanningItinerary{}, nil
}

func (s *fakePlanningService) CreatePlanningRouteDay(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, input CreatePlanningRouteDayInput) (PlanningItineraryDay, error) {
	if s.createPlanningRouteDayFunc != nil {
		return s.createPlanningRouteDayFunc(ctx, actorUserID, planningID, input)
	}
	return PlanningItineraryDay{}, nil
}

func (s *fakePlanningService) DeletePlanningRouteDay(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, dayID uuid.UUID) error {
	if s.deletePlanningRouteDayFunc != nil {
		return s.deletePlanningRouteDayFunc(ctx, actorUserID, planningID, dayID)
	}
	return nil
}

func (s *fakePlanningService) GetPlanningRouteByID(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, routeID uuid.UUID) (PlanningItineraryStop, error) {
	if s.getPlanningRouteByIDFunc != nil {
		return s.getPlanningRouteByIDFunc(ctx, actorUserID, planningID, routeID)
	}
	return PlanningItineraryStop{}, nil
}

func (s *fakePlanningService) CreatePlanningRouteStop(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, input CreatePlanningRouteStopInput) (PlanningItineraryStop, error) {
	if s.createPlanningRouteStopFunc != nil {
		return s.createPlanningRouteStopFunc(ctx, actorUserID, planningID, input)
	}
	return PlanningItineraryStop{}, nil
}

func (s *fakePlanningService) UpdatePlanningRouteStop(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, routeID uuid.UUID, input UpdatePlanningRouteStopInput) (PlanningItineraryStop, error) {
	if s.updatePlanningRouteStopFunc != nil {
		return s.updatePlanningRouteStopFunc(ctx, actorUserID, planningID, routeID, input)
	}
	return PlanningItineraryStop{}, nil
}

func (s *fakePlanningService) DeletePlanningRouteStop(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, routeID uuid.UUID) error {
	if s.deletePlanningRouteStopFunc != nil {
		return s.deletePlanningRouteStopFunc(ctx, actorUserID, planningID, routeID)
	}
	return nil
}

func (s *fakePlanningService) ReorderPlanningRouteStops(ctx context.Context, actorUserID uuid.UUID, planningID uuid.UUID, input ReorderPlanningRouteStopsInput) error {
	if s.reorderPlanningRouteStopsFunc != nil {
		return s.reorderPlanningRouteStopsFunc(ctx, actorUserID, planningID, input)
	}
	return nil
}

func registeredClaims(userID uuid.UUID) jwt.RegisteredClaims {
	return jwt.RegisteredClaims{Subject: userID.String()}
}
