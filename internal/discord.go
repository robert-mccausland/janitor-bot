package internal

import (
	"fmt"
	"os"

	"github.com/bwmarrin/discordgo"
)

func SetupDiscordClient() (*discordgo.Session, error) {
	token := os.Getenv("TOKEN")
	if token == "" {
		return nil, fmt.Errorf("TOKEN environment variable must be set")
	}

	s, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("error creating Discord session: %v", err)
	}

	ready := make(chan bool)
	s.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		ready <- true
	})

	s.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildVoiceStates
	err = s.Open()
	if err != nil {
		return nil, fmt.Errorf("error opening connection: %v", err)
	}

	<-ready

	return s, nil
}

func ShutdownDiscordClient(s *discordgo.Session) error {
	err := s.Close()
	if err != nil {
		return fmt.Errorf("error while closing discord connection: %v", err)
	}

	return nil
}
