package controller

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/EgorGapo/bank/internal/domain"
)

func TestRespondError(t *testing.T) {
	// Table-driven: один набор проверок на все ветки switch в respondError.
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"invalid uuid", ErrInvalidUUID, http.StatusBadRequest, codeInvalidRequest},
		{"invalid body", ErrInvalidBody, http.StatusBadRequest, codeInvalidRequest},
		{"invalid amount", ErrInvalidAmount, http.StatusBadRequest, codeInvalidRequest},
		{"same account", ErrSameTransferAccount, http.StatusBadRequest, codeInvalidRequest},
		{"account not found", domain.ErrAccountNotFound, http.StatusNotFound, codeAccountNotFound},
		{"not enough money", domain.ErrNotEnoughMoney, http.StatusUnprocessableEntity, codeInsufficientFunds},
		{"idempotency reuse", domain.ErrIdempotencyKeyReuse, http.StatusUnprocessableEntity, codeIdempotencyKeyReuse},
		{"unknown error", errors.New("boom"), http.StatusInternalServerError, codeInternalError},

		{"wrapped domain error", errors.New("x"), http.StatusInternalServerError, codeInternalError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// httptest.NewRecorder — фейковый ResponseWriter: запоминает статус, заголовки и тело.
			rec := httptest.NewRecorder()
			// Запрос нужен, потому что respondError достаёт логгер из r.Context().
			req := httptest.NewRequest(http.MethodGet, "/v1/accounts/x", nil)

			respondError(rec, req, tt.err)

			if rec.Code != tt.wantStatus {
				t.Errorf("status: got %d, want %d", rec.Code, tt.wantStatus)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("content-type: got %q, want application/json", ct)
			}

			var body errorBody
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.Error.Code != tt.wantCode {
				t.Errorf("code: got %q, want %q", body.Error.Code, tt.wantCode)
			}
			if body.Error.Message == "" {
				t.Error("message is empty")
			}
		})
	}
}

func TestRespondErrorUnwrapsWrapped(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	wrapped := errors.Join(errors.New("transfer failed"), domain.ErrNotEnoughMoney)
	respondError(rec, req, wrapped)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status: got %d, want %d (errors.Is должен размотать обёртку)",
			rec.Code, http.StatusUnprocessableEntity)
	}
}
