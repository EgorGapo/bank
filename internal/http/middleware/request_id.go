package middleware

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
)

type ctxKey int

const loggerKey ctxKey = 0

func RequestID(base *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get("X-Request-Id") // пришёл от nginx — используем его
			if id == "" {
				id = uuid.NewString() // не пришёл — генерим сами
			}
			w.Header().Set("X-Request-Id", id) // вернём клиенту, чтобы мог сослаться

			logger := base.With("request_id", id)
			ctx := context.WithValue(r.Context(), loggerKey, logger)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// FromContext достаёт обогащённый логгер; фолбэк — чтобы код без middleware не падал.
func FromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(loggerKey).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}
