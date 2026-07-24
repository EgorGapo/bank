package usecases

import (
	"context"
	"log/slog"

	"github.com/EgorGapo/bank/internal/domain"
	"github.com/EgorGapo/bank/internal/logging"
	"github.com/google/uuid"
)

type BankStorage interface {
	CreateAccount(ctx context.Context, account *domain.Account) error
	GetAccount(ctx context.Context, id string) (*domain.Account, error)
	Deposit(ctx context.Context, amount int64, transferID string, toAccountId string, idempotencyKey string) (*domain.Transfer, error)
	Withdraw(ctx context.Context, amount int64, transferID string, fromAccountID string, idempotencyKey string) (*domain.Transfer, error)
	Transfer(ctx context.Context, amount int64, transferID string, fromAccountID string, toAccountId string, idempotencyKey string) (*domain.Transfer, error)
	GetHistory(ctx context.Context, accountID string, cursor int64, limit int64) ([]domain.LedgerEntry, error)
}

type bank struct {
	storage BankStorage
	logger  *slog.Logger
}

func NewBank(storage BankStorage, logger *slog.Logger) *bank {
	return &bank{
		storage: storage,
		logger:  logger,
	}
}

func (s *bank) CreateAccount(ctx context.Context) (*domain.Account, error) {
	acc := &domain.Account{
		ID:     uuid.NewString(),
		Status: domain.StatusActive,
	}
	if err := s.storage.CreateAccount(ctx, acc); err != nil {
		return nil, err
	}
	return acc, nil
}

func (s *bank) GetAccount(ctx context.Context, id string) (*domain.Account, error) {
	return s.storage.GetAccount(ctx, id)
}

func (s *bank) Withdraw(ctx context.Context, accountID string, amount int64, idempotencyKey string) (*domain.Transfer, error) {
	transferID := uuid.NewString()
	tr, err := s.storage.Withdraw(ctx, amount, transferID, accountID, idempotencyKey)
	if err != nil {
		return nil, err
	}
	logging.FromContext(ctx).Info("withdraw completed", "transfer_id", tr.ID, "account_id", *tr.FromAccountID, "amount", tr.Amount, "status", tr.Status)
	return tr, nil
}

func (s *bank) Deposit(ctx context.Context, accountID string, amount int64, idempotencyKey string) (*domain.Transfer, error) {
	transferID := uuid.NewString()
	tr, err := s.storage.Deposit(ctx, amount, transferID, accountID, idempotencyKey)
	if err != nil {
		return nil, err
	}
	logging.FromContext(ctx).Info("deposit completed", "transfer_id", tr.ID, "account_id", *tr.ToAccountID, "amount", tr.Amount, "status", tr.Status)
	return tr, nil
}

func (s *bank) Transfer(ctx context.Context, amount int64, fromAccountID string, toAccountId string, idempotencyKey string) (*domain.Transfer, error) {
	transferID := uuid.NewString()
	tr, err := s.storage.Transfer(ctx, amount, transferID, fromAccountID, toAccountId, idempotencyKey)
	if err != nil {
		return nil, err
	}
	logging.FromContext(ctx).Info("transfer completed", "transfer_id", tr.ID, "account_id", *tr.ToAccountID, "amount", tr.Amount, "status", tr.Status)
	return tr, nil
}

func (s *bank) GetHistory(ctx context.Context, accountID string, cursor int64, limit int64) (domain.HistoryPage, error) {
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
