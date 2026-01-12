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
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	err := godotenv.Load()
	if err != nil {
		fmt.Printf("WARNING: could not load .env file: %v", err)
	}

	shutdown, err := internal.SetupOTel(ctx)
	if err != nil {
		fmt.Printf("ERROR: unable to setup OTel: %v", err)
	}

	logger := internal.NewLogger("github.com/robert-mccausland/janitor-bot/main")
	defer func() {
		err := shutdown(ctx)
		if err != nil {
			logger.Error("Unable to shutdown OTel", slog.Any("err", err))
		}
	}()

	logger.Info("janitor-bot is starting")
	s, err := internal.SetupDiscordClient()
	if err != nil {
		logger.Error("Unable to setup discord client", slog.Any("err", err))
	}
	defer func() {
		err := internal.ShutdownDiscordClient(s)
		if err != nil {
			logger.Error("Unable to shutdown discord client", slog.Any("err", err))
		}
	}()

	err = internal.Janate(s)
	if err != nil {
		logger.Error("Error while janitoring", slog.Any("err", err))
		return
	}

	logger.Info("janitor-bot has started successfully")
	<-ctx.Done()
	logger.Info("janitor-bot is shutting down")
}
