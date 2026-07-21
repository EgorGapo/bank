package usecases

import (
	"context"
	"log/slog"

	"github.com/EgorGapo/bank/internal/domain"
	"github.com/EgorGapo/bank/internal/logging"
	"github.com/google/uuid"
)

type Storage interface {
	CreateAccount(ctx context.Context, account *domain.Account) error
	GetAccount(ctx context.Context, id string) (*domain.Account, error)
	Deposit(ctx context.Context, amount int64, transferID string, toAccountId string, idempotencyKey string) (*domain.Transfer, error)
	Withdraw(ctx context.Context, amount int64, transferID string, fromAccountID string, idempotencyKey string) (*domain.Transfer, error)
	Transfer(ctx context.Context, amount int64, transferID string, fromAccountID string, toAccountId string, idempotencyKey string) (*domain.Transfer, error)
	GetHistory(ctx context.Context, accountID string, cursor int64, limit int64) ([]domain.LedgerEntry, error)
}

type Bank struct {
	storage Storage
	logger  *slog.Logger
}

func NewBank(storage Storage, logger *slog.Logger) *Bank {
	return &Bank{
		storage: storage,
		logger:  logger,
	}
}

func (s *Bank) CreateAccount(ctx context.Context) (*domain.Account, error) {
	acc := &domain.Account{
		ID:     uuid.NewString(),
		Status: domain.StatusActive,
	}
	if err := s.storage.CreateAccount(ctx, acc); err != nil {
		return nil, err
	}
	return acc, nil
}

func (s *Bank) GetAccount(ctx context.Context, id string) (*domain.Account, error) {
	return s.storage.GetAccount(ctx, id)
}

func (s *Bank) Withdraw(ctx context.Context, accountID string, amount int64, idempotencyKey string) (*domain.Transfer, error) {
	transferID := uuid.NewString()
	tr, err := s.storage.Withdraw(ctx, amount, transferID, accountID, idempotencyKey)
	if err != nil {
		return nil, err
	}
	logging.FromContext(ctx).Info("withdraw completed", "transfer_id", tr.ID, "account_id", *tr.FromAccountID, "amount", tr.Amount, "status", tr.Status)
	return tr, nil
}

func (s *Bank) Deposit(ctx context.Context, accountID string, amount int64, idempotencyKey string) (*domain.Transfer, error) {
	transferID := uuid.NewString()
	tr, err := s.storage.Deposit(ctx, amount, transferID, accountID, idempotencyKey)
	if err != nil {
		return nil, err
	}
	logging.FromContext(ctx).Info("deposit completed", "transfer_id", tr.ID, "account_id", *tr.ToAccountID, "amount", tr.Amount, "status", tr.Status)
	return tr, nil
}

func (s *Bank) Transfer(ctx context.Context, amount int64, fromAccountID string, toAccountId string, idempotencyKey string) (*domain.Transfer, error) {
	transferID := uuid.NewString()
	tr, err := s.storage.Transfer(ctx, amount, transferID, fromAccountID, toAccountId, idempotencyKey)
	if err != nil {
		return nil, err
	}
	logging.FromContext(ctx).Info("transfer completed", "transfer_id", tr.ID, "account_id", *tr.ToAccountID, "amount", tr.Amount, "status", tr.Status)
	return tr, nil
}

func (s *Bank) GetHistory(ctx context.Context, accountID string, cursor int64, limit int64) (domain.HistoryPage, error) {
	h, err := s.storage.GetHistory(ctx, accountID, cursor, limit+1)
	page := domain.HistoryPage{}
	if err != nil {
		return page, err
	}

	if len(h) > int(limit) {
		page.HasMore = true
		h = h[:limit]
		page.NextCursor = h[limit-1].ID
	}
	page.Entries = h
	return page, nil
}
