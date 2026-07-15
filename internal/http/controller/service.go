package controller

import (
	"context"
	"log/slog"

	"github.com/EgorGapo/bank/internal/domain"
)

type Bank interface {
	CreateAccount(ctx context.Context) (*domain.Account, error)
	GetAccount(ctx context.Context, id string) (*domain.Account, error)
}

type Implementation struct {
	logger   *slog.Logger
	usecases Bank
}

func New(logger *slog.Logger, usecases Bank) *Implementation {
	return &Implementation{
		logger:   logger,
		usecases: usecases,
	}
}
