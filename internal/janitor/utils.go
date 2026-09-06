package janitor

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/robert-mccausland/janitor-bot/internal/discord"
)

func moveUsers(c *discord.Client, fromChannel *discord.Channel, toChannel *discord.Channel) error {
	guild := c.Guild(fromChannel.GuildID)
	if guild == nil {
		return fmt.Errorf("guild not found: %s", fromChannel.GuildID)
	}

	var usersToMove []discord.VoiceState
	for _, vs := range guild.VoiceStates {
		if vs.ChannelID != nil && *vs.ChannelID == fromChannel.ID {
			usersToMove = append(usersToMove, vs)
		}
	}

	if len(usersToMove) == 0 {
		return nil
	}

	logger.Info("Moving people between channels", slog.String("from_channel_id", fromChannel.ID), slog.String("to_channel_id", toChannel.ID))
	vc, err := c.JoinVoiceChannel(fromChannel.GuildID, fromChannel.ID, false, false)
	if err != nil {
		return fmt.Errorf("failed to join voice channel: %v", err)
	}

	defer func() {
		err = vc.Leave()
		if err != nil {
			logger.Error("Failed to disconnect from voice: %v", slog.Any("err", err))
		}
	}()

	err = playSound(c, fromChannel, SoundConfig{SoundID: "1223777210650067056", Duration: 2 * time.Second})
	if err != nil {
		return fmt.Errorf("failed to play joining sound: %w", err)
	}

	for _, vs := range usersToMove {
		err := c.ModifyGuildMember(guild.ID, vs.UserID, discord.GuildMemberUpdate{
			ChannelID: &toChannel.ID,
		})
		if err != nil {
			return fmt.Errorf("error moving user %s: %v", vs.UserID, err)
		}
	}

	err = vc.ChangeChannel(toChannel.ID, false, false)
	if err != nil {
		return fmt.Errorf("failed to switch channels: %v", err)
	}

	err = playSound(c, toChannel, SoundConfig{SoundID: "1449829431693672641", Duration: 1 * time.Second})
	if err != nil {
		return fmt.Errorf("failed to play leaving sound: %v", err)
	}

	return nil
}

type SoundConfig struct {
	SoundID  string
	Duration time.Duration
}

func playSound(c *discord.Client, channel *discord.Channel, data SoundConfig) error {
	failures := 0
	for {
		err := c.SendSoundboardSound(channel.ID, discord.SendSoundboardSound{SoundID: data.SoundID})
		if err != nil {
			var apiError *discord.DiscordAPIError
			if errors.As(err, &apiError) && apiError.Code == 50168 {
				failures++
				if failures >= 3 {
					return err
				}
				logger.Warn(fmt.Sprintf("got error code 50168, retrying in 1 second: %v", apiError), slog.Any("error", err), slog.Int("failures", failures))
				time.Sleep(1 * time.Second)
				continue
			}
			return err
		}

		break
	}

	time.Sleep(data.Duration)
	return nil
}
