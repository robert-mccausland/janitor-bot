package internal

import "github.com/bwmarrin/discordgo"

type SoundboardSoundSend struct {
	// The ID of the sound to send
	SoundID string `json:"sound_id"`

	// Guild ID of the sound to send, if it is a guild sound
	// Required to send a sound from another guild
	GuildID string `json:"guild_id,omitempty"`
}

func SendSoundboardSound(s *discordgo.Session, channelID string, data SoundboardSoundSend, options ...discordgo.RequestOption) error {
	endpoint := discordgo.EndpointChannel(channelID) + "/send-soundboard-sound"
	res, err := s.RequestWithBucketID("POST", endpoint, data, endpoint, options...)
	println(res)
	return err
}
