package route

import (
	"log/slog"

	"github.com/EgorGapo/bank/internal/http/controller"
	"github.com/EgorGapo/bank/internal/http/middleware"
	"github.com/go-chi/chi/v5"
)

func Setup(r chi.Router, handler *controller.Implementation, logger *slog.Logger) {
	r.Use(middleware.RequestID(logger))
	r.Use(middleware.AccessLog)

	r.Route("/v1", func(r chi.Router) {
		r.Post("/accounts", handler.CreateAccountHandler)
		r.Get("/accounts/{id}", handler.GetAccountHandler)
	})
}
