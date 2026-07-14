package main

import (
	"log"
	"log/slog"
	"os"

	"github.com/EgorGapo/bank/internal/app"
	"github.com/EgorGapo/bank/internal/config"
)

func main() {
	cfg, err := config.New()
	if err != nil {
		log.Fatalf("can not get application config: %s", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	if err := app.Run(cfg, logger); err != nil {
		logger.Error("service failed", "error", err)
		os.Exit(1)
	}
}
