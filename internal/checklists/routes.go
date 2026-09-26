package checklists

import "github.com/go-chi/chi/v5"

func (h *Handler) RegisterRoutes(router chi.Router) {
	router.Route("/plannings/{planningId}/checklist-tasks", func(checklistRouter chi.Router) {
		checklistRouter.Get("/", h.listTasks)
		checklistRouter.Post("/", h.createTask)
		checklistRouter.Get("/{taskId}", h.getTask)
		checklistRouter.Patch("/{taskId}", h.updateTask)
		checklistRouter.Delete("/{taskId}", h.deleteTask)
		checklistRouter.Post("/{taskId}/toggle-completion", h.toggleTaskCompletion)
	})
}
