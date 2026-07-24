package main

import (
	"log"
	"log/slog"
	"os"

	"github.com/EgorGapo/bank/internal/config"
	"github.com/EgorGapo/bank/internal/notifier"
)

func main() {
	cfg, err := config.New()
	if err != nil {
		log.Fatalf("can not get application config: %s", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	if err := notifier.Run(logger, cfg); err != nil {
		logger.Error("service failed", "error", err)
		os.Exit(1)
	}

}
