package app

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"

	_ "github.com/go-sql-driver/mysql"

	"github.com/DoWithLogic/go-echo-realworld/config"
	"github.com/DoWithLogic/go-echo-realworld/pkg/datasource"
	"github.com/DoWithLogic/go-echo-realworld/pkg/httpx"
	"github.com/DoWithLogic/go-echo-realworld/pkg/otel/zerolog"
	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
)

type App struct {
	DB     *sqlx.DB
	Router *chi.Mux
	Log    *zerolog.Logger
	Cfg    config.Config
}

func NewApp(ctx context.Context, cfg config.Config) *App {
	db, err := datasource.NewDatabase(cfg.Database)
	if err != nil {
		panic(err)
	}

	return &App{
		DB:     db,
		Router: httpx.NewRouter(cfg.Server.Debug),
		Log:    zerolog.NewZeroLog(ctx, os.Stdout),
		Cfg:    cfg,
	}
}

func (app *App) Start() error {
	if err := app.StartService(); err != nil {
		app.Log.Z().Err(err).Msg("[app]StartService")

		return err
	}

	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", app.Cfg.Server.Port),
		ReadTimeout:  app.Cfg.Server.ReadTimeout,
		WriteTimeout: app.Cfg.Server.WriteTimeout,
		Handler:      app.Router,
	}

	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return err
	}

	fmt.Printf("⇨ http server started on %s\n", listener.Addr())

	return server.Serve(listener)
}
