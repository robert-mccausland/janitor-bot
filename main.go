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
	}

	logger := logging.NewLogger("github.com/robert-mccausland/janitor-bot/main")
	defer func() {
		err := shutdown(ctx)
		if err != nil {
			logger.Error("Unable to shutdown OTel", slog.Any("err", err))
		}
	}()

	logger.Info("janitor-bot is starting")
	client, err := internal.SetupDiscordClient(ctx)
	if err != nil {
		logger.Error("Unable to setup discord client", slog.Any("err", err))
	}
	defer func() {
		err := client.WaitForShutdown()
		if err != nil {
			logger.Error("Error while waiting for discord client to shutdown", slog.Any("err", err))
		}
	}()

	err = internal.Janate(client, ctx)
	if err != nil {
		logger.Error("Error while janitoring", slog.Any("err", err))
		return
	}

	logger.Info("janitor-bot has started successfully")
	<-ctx.Done()
	logger.Info("janitor-bot is shutting down")
}
