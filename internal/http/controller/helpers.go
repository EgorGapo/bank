package controller

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/EgorGapo/bank/internal/domain"
	"github.com/EgorGapo/bank/internal/logging"
)

var ErrInvalidUUID = errors.New("invalid uuid format")
var ErrInvalidAmount = errors.New("invalid amount")
var ErrInvalidBody = errors.New("invalid body")
var ErrInvalidIdempotencyKey = errors.New("invalid idempotency key")
var ErrSameTransferAccount = errors.New("same transfer account")
var ErrInvalidLimitFormat = errors.New("wrong limit format")
var ErrInvalidCursorFormat = errors.New("wrong cursor format")

const (
	codeInvalidRequest      = "invalid_request"
	codeAccountNotFound     = "account_not_found"
	codeInsufficientFunds   = "insufficient_funds"
	codeIdempotencyKeyReuse = "idempotency_key_reuse"
	codeInternalError       = "internal_error"
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
	logger := logging.FromContext(r.Context())

	switch {
	case errors.Is(err, ErrInvalidUUID):
		writeError(w, http.StatusBadRequest, codeInvalidRequest, err.Error())

	case errors.Is(err, ErrInvalidIdempotencyKey):
		writeError(w, http.StatusBadRequest, codeInvalidRequest, err.Error())

	case errors.Is(err, domain.ErrAccountNotFound):
		writeError(w, http.StatusNotFound, codeAccountNotFound, err.Error())

	case errors.Is(err, ErrInvalidAmount) || errors.Is(err, ErrInvalidBody) ||
		errors.Is(err, ErrSameTransferAccount) ||
		errors.Is(err, ErrInvalidLimitFormat) || errors.Is(err, ErrInvalidCursorFormat):
		writeError(w, http.StatusBadRequest, codeInvalidRequest, err.Error())

	case errors.Is(err, domain.ErrNotEnoughMoney):
		writeError(w, http.StatusUnprocessableEntity, codeInsufficientFunds, err.Error())

	case errors.Is(err, domain.ErrIdempotencyKeyReuse):
		writeError(w, http.StatusUnprocessableEntity, codeIdempotencyKeyReuse, err.Error())

	default:
		logger.Error("internal error", "error", err)
		writeError(w, http.StatusInternalServerError, codeInternalError, "internal server error")
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	var body errorBody
	body.Error.Code = code
	body.Error.Message = message
	respondJSON(w, status, body)
}
