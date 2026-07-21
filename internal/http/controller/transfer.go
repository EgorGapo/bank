package controller

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type transferRequest struct {
	FromAccount string `json:"from"`
	ToAccount   string `json:"to"`
	Amount      int64  `json:"amount"`
}

type transferResponse struct {
	ID            string     `json:"id"`
	FromAccountID *string    `json:"from_account_id,omitempty"`
	ToAccountID   *string    `json:"to_account_id,omitempty"`
	Type          string     `json:"type"`
	Amount        int64      `json:"amount"`
	Status        string     `json:"status"`
	CreatedAt     time.Time  `json:"created_at"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
}

func (s *Implementation) TransferHandler(w http.ResponseWriter, r *http.Request) {
	key := r.Header.Get("Idempotency-Key")
	if _, err := uuid.Parse(key); err != nil {
		respondError(w, r, ErrInvalidIdempotencyKey)
		return
	}
	req := transferRequest{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, r, ErrInvalidBody)
		return
	}
	if _, err := uuid.Parse(req.FromAccount); err != nil {
		respondError(w, r, ErrInvalidUUID)
		return
	}
	if _, err := uuid.Parse(req.ToAccount); err != nil {
		respondError(w, r, ErrInvalidUUID)
		return
	}

	if req.FromAccount == req.ToAccount {
		respondError(w, r, ErrSameTransferAccount)
		return
	}
	if req.Amount <= 0 {
		respondError(w, r, ErrInvalidAmount)
		return
	}

	transfer, err := s.usecases.Transfer(r.Context(), req.Amount, req.FromAccount, req.ToAccount, key)
	if err != nil {
		respondError(w, r, err)
		return
	}

	respondJSON(w, http.StatusCreated, transferResponse{
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
