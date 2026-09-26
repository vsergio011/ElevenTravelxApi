package users

import "github.com/go-chi/chi/v5"

func (h *Handler) RegisterRoutes(router chi.Router) {
	router.Route("/users", func(usersRouter chi.Router) {
		usersRouter.Get("/me/profile", h.getMyProfile)
		usersRouter.Patch("/me/profile", h.updateMyProfile)
		usersRouter.Get("/{username}/profile", h.getProfileByUsername)

		usersRouter.Post("/{userId}/follow", h.followUser)
		usersRouter.Delete("/{userId}/follow", h.unfollowUser)
		usersRouter.Get("/{userId}/followers", h.listFollowers)
		usersRouter.Get("/{userId}/following", h.listFollowing)

		usersRouter.Post("/{userId}/locations", h.createLocation)
		usersRouter.Patch("/{userId}/locations/{locationId}", h.updateLocation)
		usersRouter.Delete("/{userId}/locations/{locationId}", h.deleteLocation)
		usersRouter.Post("/{userId}/media", h.createMedia)
		usersRouter.Post("/{userId}/achievements/verify", h.verifyAchievement)
	})
}
