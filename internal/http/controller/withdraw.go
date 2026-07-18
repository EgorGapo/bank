package controller

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type withdrawRequest struct {
	Amount int64 `json:"amount"`
}

type withdrawResponse struct {
	ID            string     `json:"id"`
	FromAccountID *string    `json:"from_account_id,omitempty"`
	ToAccountID   *string    `json:"to_account_id,omitempty"`
	Type          string     `json:"type"`
	Amount        int64      `json:"amount"`
	Status        string     `json:"status"`
	CreatedAt     time.Time  `json:"created_at"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
}

func (s *Implementation) WithdrawHandler(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "id")
	if _, err := uuid.Parse(accountID); err != nil {
		respondError(w, r, ErrInvalidUUID)
		return
	}
	key := r.Header.Get("Idempotency-Key")
	if _, err := uuid.Parse(key); err != nil {
		respondError(w, r, ErrInvalidIdempotencyKey)
		return
	}
	req := withdrawRequest{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, r, ErrInvalidBody)
		return
	}
	if req.Amount <= 0 {
		respondError(w, r, ErrInvalidAmount)
		return
	}

	transfer, err := s.usecases.Withdraw(r.Context(), accountID, req.Amount, key)
	if err != nil {
		respondError(w, r, err)
		return
	}

	respondJSON(w, http.StatusCreated, withdrawResponse{
		ID:            transfer.ID,
		FromAccountID: transfer.FromAccountID,
		ToAccountID:   transfer.ToAccountID,
		Type:          transfer.Type,
		Amount:        transfer.Amount,
		Status:        transfer.Status,
		CreatedAt:     transfer.CreatedAt,
		CompletedAt:   transfer.CompletedAt,
	})

}
