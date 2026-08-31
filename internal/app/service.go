package app

import (
	userV1 "github.com/DoWithLogic/go-echo-realworld/internal/users/delivery/http/v1"
	userRepository "github.com/DoWithLogic/go-echo-realworld/internal/users/repository"
	userUseCase "github.com/DoWithLogic/go-echo-realworld/internal/users/usecase"
	"github.com/go-chi/chi/v5"
)

func (app *App) StartService() error {
	// define repository
	userRepo := userRepository.NewRepository(app.DB, app.Log)

	// define usecase
	userUC := userUseCase.NewUseCase(userRepo, app.Log, app.Cfg)

	// define controllers
	userCTRL := userV1.NewHandlers(userUC, app.Log)

	app.Router.Route("/api/v1", func(version chi.Router) {
		userV1.MapUserRoute(version, userCTRL, app.Cfg)
	})

	return nil
}
