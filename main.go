package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/robert-mccausland/janitor-bot/internal"
	"github.com/robert-mccausland/janitor-bot/internal/janitor"
	"github.com/robert-mccausland/janitor-bot/internal/logging"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	err := godotenv.Load()
	if err != nil {
		fmt.Printf("WARNING: could not load .env file: %v", err)
	}

	shutdown, err := logging.SetupLogging(ctx)
	if err != nil {
		fmt.Printf("ERROR: unable to setup OTel: %v", err)
		return
	}

	logger := logging.NewLogger("github.com/robert-mccausland/janitor-bot/main")
	defer func() {
		err := shutdown(ctx)
		if err != nil {
			logger.Error(fmt.Sprintf("Unable to shutdown OTel: %v", err), slog.Any("err", err))
		}
	}()
	logger.Info("janitor-bot is starting")

	config, err := internal.GetConfig()
	if err != nil {
		logger.Error(fmt.Sprintf("Error loading config: %v", err), slog.Any("err", err))
		return
	}

	janitor, err := janitor.NewJanitor(ctx, *config)
	if err != nil {
		logger.Error(fmt.Sprintf("Error while creating janitor: %v", err), slog.Any("err", err))
		return
	}

	err = janitor.Janate()
	if err != nil {
		logger.Error(fmt.Sprintf("Error while janitoring: %v", err), slog.Any("err", err))
		return
	}
	logger.Info("janitor-bot is shutting down")
}
