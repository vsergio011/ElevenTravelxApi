package expenses

import "github.com/go-chi/chi/v5"

func (h *Handler) RegisterRoutes(router chi.Router) {
	router.Get("/plannings/{planningId}/budget", h.getBudget)
	router.Route("/plannings/{planningId}/expenses", func(expenseRouter chi.Router) {
		expenseRouter.Get("/", h.listExpenses)
		expenseRouter.Post("/", h.createExpense)
		expenseRouter.Get("/{expenseId}", h.getExpense)
		expenseRouter.Patch("/{expenseId}", h.updateExpense)
		expenseRouter.Delete("/{expenseId}", h.deleteExpense)
	})
}
