package route

import (
	"github.com/EgorGapo/bank/internal/http/controller"
	"github.com/go-chi/chi/v5"
)

func Setup(r chi.Router, handler *controller.Implementation) {
	r.Route("/v1", func(r chi.Router) {
		r.Post("/accounts", handler.CreateAccountHandler)
	})
}
