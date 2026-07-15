package controller

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type getAccountResponse struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	Balance   int64     `json:"balance"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (s *Implementation) GetAccountHandler(w http.ResponseWriter, r *http.Request) {
	accID := chi.URLParam(r, "id")
	if _, err := uuid.Parse(accID); err != nil {
		respondError(w, r, ErrInvalidUUID)
		return
	}
	account, err := s.usecases.GetAccount(r.Context(), accID)
	if err != nil {
		respondError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, getAccountResponse{
		ID:        account.ID,
		Status:    account.Status,
		Balance:   account.Balance,
		CreatedAt: account.CreatedAt,
		UpdatedAt: account.UpdatedAt,
	})

}
