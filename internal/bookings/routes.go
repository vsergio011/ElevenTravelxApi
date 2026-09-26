package bookings

import "github.com/go-chi/chi/v5"

func (h *Handler) RegisterRoutes(router chi.Router) {
	router.Route("/plannings/{planningId}/bookings", func(bookingsRouter chi.Router) {
		bookingsRouter.Get("/budget", h.getBudget)
		bookingsRouter.Get("/", h.listBookings)
		bookingsRouter.Post("/", h.createBooking)
		bookingsRouter.Get("/{bookingId}", h.getBooking)
		bookingsRouter.Patch("/{bookingId}", h.updateBooking)
		bookingsRouter.Post("/{bookingId}/confirm", h.confirmBooking)
		bookingsRouter.Delete("/{bookingId}", h.deleteBooking)
	})
}
