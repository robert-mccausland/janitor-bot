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
	client, err := internal.SetupDiscordClient(ctx)
	if err != nil {
		logger.Error(fmt.Sprintf("Unable to setup discord client: %v", err), slog.Any("err", err))
		return
	}

	err = internal.Janate(client, ctx)
	if err != nil {
		logger.Error(fmt.Sprintf("Error while janitoring: %v", err), slog.Any("err", err))
		return
	}

	logger.Info("janitor-bot has started successfully")

	select {
	case err := <-ctx.Done():
		logger.Info(fmt.Sprintf("Recieved shutdown signal: %v", err), slog.Any("err", err))
	case err := <-client.Error():
		logger.Error(fmt.Sprintf("Error while running discord client: %v", err), slog.Any("err", err))
	}

	logger.Info("janitor-bot is shutting down")
}
