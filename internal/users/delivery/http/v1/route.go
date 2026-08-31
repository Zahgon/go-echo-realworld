package v1

import (
	"github.com/DoWithLogic/go-echo-realworld/config"
	"github.com/DoWithLogic/go-echo-realworld/pkg/httpx"
	"github.com/DoWithLogic/go-echo-realworld/pkg/middleware"
	"github.com/go-chi/chi/v5"
)

// userRoutePatterns lists every path this module serves. They also answer
// OPTIONS with the set of methods registered for them.
var userRoutePatterns = []string{
	"/users",
	"/users/login",
	"/user",
	"/profiles/{username}",
	"/profiles/{username}/follow",
}

func MapUserRoute(version chi.Router, h Handlers, cfg config.Config) {
	version.Post("/users", h.CreateUser)
	version.Post("/users/login", h.Login)
	version.With(middleware.AuthorizeJWT(cfg)).Get("/user", h.UserDetail)
	version.With(middleware.AuthorizeJWT(cfg)).Put("/user", h.UpdateUser)
	version.With(middleware.OptionalAuthJWT(cfg)).Get("/profiles/{username}", h.ProfileDetail)
	version.With(middleware.AuthorizeJWT(cfg)).Post("/profiles/{username}/follow", h.ProfileFollowUser)

	for _, pattern := range userRoutePatterns {
		version.Options(pattern, httpx.OptionsHandler)
	}
}
