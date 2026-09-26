package plannings

import "github.com/go-chi/chi/v5"

func (h *Handler) RegisterRoutes(router chi.Router) {
	router.Route("/plannings", func(planningsRouter chi.Router) {
		planningsRouter.Get("/", h.listPlannings)
		planningsRouter.Post("/", h.createPlanning)
		planningsRouter.Get("/{planningId}", h.getPlanning)
		planningsRouter.Get("/{planningId}/summary", h.getSummary)
		planningsRouter.Patch("/{planningId}", h.updatePlanning)
		planningsRouter.Post("/{planningId}/archive", h.archivePlanning)
		planningsRouter.Post("/{planningId}/unarchive", h.unarchivePlanning)
		planningsRouter.Get("/{planningId}/members", h.listMembers)
		planningsRouter.Post("/{planningId}/members", h.addMember)
		planningsRouter.Patch("/{planningId}/members/{userId}", h.updateMemberRole)
		planningsRouter.Delete("/{planningId}/members/{userId}", h.removeMember)
		planningsRouter.Get("/{planningId}/activity", h.listActivity)
		planningsRouter.Get("/{planningId}/routes", h.getPlanningRoutes)
		planningsRouter.Post("/{planningId}/route-days", h.createPlanningRouteDay)
		planningsRouter.Delete("/{planningId}/route-days/{dayId}", h.deletePlanningRouteDay)
		planningsRouter.Post("/{planningId}/routes", h.createPlanningRoute)
		planningsRouter.Post("/{planningId}/routes/reorder", h.reorderPlanningRoutes)
		planningsRouter.Get("/{planningId}/routes/{routeId}", h.getPlanningRoute)
		planningsRouter.Patch("/{planningId}/routes/{routeId}", h.updatePlanningRoute)
		planningsRouter.Delete("/{planningId}/routes/{routeId}", h.deletePlanningRoute)
	})

	router.Get("/groups/{groupId}/plannings", h.listGroupPlannings)
}
