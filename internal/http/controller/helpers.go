package controller

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/EgorGapo/bank/internal/domain"
	"github.com/EgorGapo/bank/internal/http/middleware"
)

var ErrInvalidUUID = errors.New("invalid uuid format")

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
	case errors.Is(err, ErrInvalidUUID):
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())

	case errors.Is(err, domain.ErrAccountNotFound):
		writeError(w, http.StatusNotFound, "account_not_found", err.Error())

	default:
		logger.Error("internal error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	var body errorBody
	body.Error.Code = code
	body.Error.Message = message
	respondJSON(w, status, body)
}
