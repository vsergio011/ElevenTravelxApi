package notifications

import "github.com/go-chi/chi/v5"

func (h *Handler) RegisterRoutes(router chi.Router) {
	router.Route("/notifications", func(notificationsRouter chi.Router) {
		notificationsRouter.Get("/", h.listNotifications)
		notificationsRouter.Get("/unread-count", h.getUnreadCount)
		notificationsRouter.Get("/push-config", h.getPushConfig)
		notificationsRouter.Post("/read-all", h.markAllRead)
		notificationsRouter.Patch("/{notificationId}/read", h.markRead)
		notificationsRouter.Post("/push-subscriptions", h.upsertPushSubscription)
		notificationsRouter.Delete("/push-subscriptions", h.deletePushSubscription)
	})
}
