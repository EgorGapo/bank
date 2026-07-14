package controller

import (
	"context"
	"log/slog"
)

type Bank interface {
	CreateAccount(ctx context.Context)
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
