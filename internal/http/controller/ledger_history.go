package controller

import (
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const (
	defaultHistoryLimit = 20
	maxHistoryLimit     = 100
)

type ledgerEntryResponse struct {
	ID           int64     `json:"id"`
	TransferID   string    `json:"transfer_id"`
	Amount       int64     `json:"amount"`
	BalanceAfter int64     `json:"balance_after"`
	CreatedAt    time.Time `json:"created_at"`
}

type getLedgerHistoryResponse struct {
	Entries    []ledgerEntryResponse `json:"entries"`
	NextCursor string                `json:"next_cursor,omitempty"`
	HasMore    bool                  `json:"has_more"`
}

func (s *Implementation) GetLedgerHistoryHandler(w http.ResponseWriter, r *http.Request) {
	accID := chi.URLParam(r, "id")
	if _, err := uuid.Parse(accID); err != nil {
		respondError(w, r, ErrInvalidUUID)
		return
	}
	limit := int64(defaultHistoryLimit)
	if raw := r.URL.Query().Get("limit"); raw != "" {
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || v < 1 {
			respondError(w, r, ErrInvalidLimitFormat)
			return
		}
		limit = v
	}
	if limit > maxHistoryLimit {
		limit = maxHistoryLimit
	}

	cursor := int64(math.MaxInt64)
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || v < 0 {
			respondError(w, r, ErrInvalidCursorFormat)
			return
		}
		cursor = v
	}

	page, err := s.usecases.GetHistory(r.Context(), accID, cursor, limit)
	if err != nil {
		respondError(w, r, err)
		return
	}

	entries := make([]ledgerEntryResponse, 0, len(page.Entries))
	for _, e := range page.Entries {
		entries = append(entries, ledgerEntryResponse{
			ID:           e.ID,
			TransferID:   e.TransferID,
			Amount:       e.Amount,
			BalanceAfter: e.BalanceAfter,
			CreatedAt:    e.CreatedAt,
		})
	}

	resp := getLedgerHistoryResponse{Entries: entries, HasMore: page.HasMore}
	if page.HasMore {
		resp.NextCursor = strconv.FormatInt(page.NextCursor, 10)
	}
	respondJSON(w, http.StatusOK, resp)
}
