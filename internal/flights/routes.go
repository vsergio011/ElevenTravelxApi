package flights

import "github.com/go-chi/chi/v5"

func (h *Handler) RegisterRoutes(router chi.Router) {
	router.Route("/plannings/{planningId}/flights", func(r chi.Router) {
		r.Get("/", h.list)
		r.Get("/lookup", h.lookup)
		r.Post("/", h.create)
		r.Get("/{flightId}", h.get)
		r.Patch("/{flightId}", h.update)
		r.Post("/{flightId}/confirm", h.confirm)
		r.Delete("/{flightId}", h.delete)
	})
}
