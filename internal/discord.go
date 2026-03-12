package internal

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/robert-mccausland/janitor-bot/internal/discord"
)

func SetupDiscordClient(ctx context.Context) (*discord.Client, error) {
	token := os.Getenv("TOKEN")
	if token == "" {
		return nil, fmt.Errorf("TOKEN environment variable must be set")
	}

	client := discord.NewDiscordClient(discord.DefaultOptions())

	go func() {
		err := client.Run(ctx, token, int(discord.IntentGuilds|discord.IntentGuildVoiceStates))
		if err != nil && err != context.Canceled {
			logger.Error(fmt.Sprintf("Error while running discord client: %v", err), slog.Any("error", err))
		}
	}()

	err := client.WaitForReady()
	if err != nil {
		return nil, fmt.Errorf("error starting discord client: %w", err)
	}

	return client, nil
}
