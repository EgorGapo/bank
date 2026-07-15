package usecases

import (
	"context"
	"log/slog"

	"github.com/EgorGapo/bank/internal/domain"
	"github.com/google/uuid"
)

type Storage interface {
	CreateAccount(ctx context.Context, account *domain.Account) error
	GetAccount(ctx context.Context, id string) (*domain.Account, error)
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
