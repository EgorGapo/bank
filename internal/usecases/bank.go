package usecases

import (
	"context"
	"log/slog"
)

type Storage interface {
	CreateAccount(ctx context.Context)
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

func (s *Bank) CreateAccount(ctx context.Context) {
	s.storage.CreateAccount(ctx)
	panic("not implemented")
}
