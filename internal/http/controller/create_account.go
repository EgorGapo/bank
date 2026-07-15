package controller

import (
	"net/http"
	"time"
)

type createAccountResponse struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	Balance   int64     `json:"balance"`
	CreatedAt time.Time `json:"createdAt"`
}

func (s *Implementation) CreateAccountHandler(w http.ResponseWriter, r *http.Request) {
	acc, err := s.usecases.CreateAccount(r.Context())
	if err != nil {
		respondError(w, r, err)
		return
	}

	respondJSON(w, http.StatusCreated, createAccountResponse{
		ID:        acc.ID,
		Status:    acc.Status,
		Balance:   acc.Balance,
		CreatedAt: acc.CreatedAt,
	})
}
