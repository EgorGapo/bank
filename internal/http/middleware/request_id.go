package middleware

import (
	"log/slog"
	"net/http"

	"github.com/EgorGapo/bank/internal/logging"
	"github.com/google/uuid"
)

func RequestID(base *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get("X-Request-Id") // пришёл от nginx — используем его
			if id == "" {
				id = uuid.NewString() // не пришёл — генерим сами
			}
			w.Header().Set("X-Request-Id", id) // вернём клиенту, чтобы мог сослаться

			logger := base.With("request_id", id)
			ctx := logging.WithLogger(r.Context(), logger)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
