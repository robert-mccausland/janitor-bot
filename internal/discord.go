package internal

import (
	"context"
	"fmt"
	"os"

	"github.com/robert-mccausland/janitor-bot/internal/discord"
)

func SetupDiscordClient(ctx context.Context) (*discord.Client, error) {
	token := os.Getenv("TOKEN")
	if token == "" {
		return nil, fmt.Errorf("TOKEN environment variable must be set")
	}

	client := discord.NewDiscordClient(discord.DefaultOptions())
	err := client.Start(ctx, token, discord.IntentGuilds|discord.IntentGuildVoiceStates)
	if err != nil {
		return nil, fmt.Errorf("error starting discord client: %w", err)
	}

	return client, nil
}
