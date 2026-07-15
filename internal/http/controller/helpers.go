package controller

import (
	"encoding/json"
	"net/http"

	"github.com/EgorGapo/bank/internal/http/middleware"
)

func respondJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func respondError(w http.ResponseWriter, r *http.Request, err error) {
	logger := middleware.FromContext(r.Context())

	switch {
	// сюда добавятся errors.Is(err, domain.ErrAccountNotFound) → 404 и т.д.
	default:
		logger.Error("internal error", "error", err)
		body := errorBody{}
		body.Error.Code = "internal_error"
		body.Error.Message = "internal server error"
		respondJSON(w, http.StatusInternalServerError, body)
	}
}
