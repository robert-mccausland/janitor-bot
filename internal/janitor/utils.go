package janitor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/robert-mccausland/janitor-bot/internal/discord"
)

// voiceSlot stops two jobs using the janitor's voice connection at once, as it
// can only be in one channel per guild. Take it with acquireVoice.
var voiceSlot = make(chan struct{}, 1)

// acquireVoice waits for the voice connection to be free, or for ctx to end.
// Call releaseVoice when done with it.
func acquireVoice(ctx context.Context) error {
	select {
	case voiceSlot <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func releaseVoice() { <-voiceSlot }

func moveUsers(c *discord.Client, fromChannel *discord.Channel, toChannel *discord.Channel) error {
	_, err := moveSomeUsers(context.Background(), c, fromChannel, toChannel, nil, nil)
	return err
}

// moveSomeUsers moves the users in only (or everyone, if only is nil) from
// fromChannel to toChannel, or disconnects them if toChannel is nil. It returns
// the IDs of the users it moved. If skip is set, it's checked once the voice
// connection is free, and nobody is moved if it returns true.
func moveSomeUsers(ctx context.Context, c *discord.Client, fromChannel *discord.Channel, toChannel *discord.Channel, only map[string]bool, skip func() bool) ([]string, error) {
	if err := acquireVoice(ctx); err != nil {
		return nil, err
	}
	defer releaseVoice()

	if skip != nil && skip() {
		return nil, nil
	}

	guild := c.Guild(fromChannel.GuildID)
	if guild == nil {
		return nil, fmt.Errorf("guild not found: %s", fromChannel.GuildID)
	}

	var usersToMove []discord.VoiceState
	for _, vs := range guild.VoiceStates {
		if vs.ChannelID != nil && *vs.ChannelID == fromChannel.ID && vs.UserID != c.User().ID && (only == nil || only[vs.UserID]) {
			usersToMove = append(usersToMove, vs)
		}
	}

	if len(usersToMove) == 0 {
		return nil, nil
	}

	toChannelID := ""
	if toChannel != nil {
		toChannelID = toChannel.ID
	}
	logger.Info("Moving people between channels", slog.String("from_channel_id", fromChannel.ID), slog.String("to_channel_id", toChannelID))
	vc, err := c.JoinVoiceChannel(fromChannel.GuildID, fromChannel.ID, false, false)
	if err != nil {
		return nil, fmt.Errorf("failed to join voice channel: %v", err)
	}

	defer func() {
		err = vc.Leave()
		if err != nil {
			logger.Error(fmt.Sprintf("Failed to disconnect from voice: %v", err), slog.Any("err", err))
		}
	}()

	err = playSound(c, fromChannel, SoundConfig{SoundID: "1223777210650067056", Duration: 2 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("failed to play joining sound: %w", err)
	}

	var moved []string
	for _, vs := range usersToMove {
		err := c.ModifyGuildMember(guild.ID, vs.UserID, discord.GuildMemberUpdate{
			ChannelID: &toChannelID,
		})
		if err != nil {
			return moved, fmt.Errorf("error moving user %s: %v", vs.UserID, err)
		}
		moved = append(moved, vs.UserID)
	}

	if toChannel == nil {
		return moved, nil
	}

	err = vc.ChangeChannel(toChannel.ID, false, false)
	if err != nil {
		return moved, fmt.Errorf("failed to switch channels: %v", err)
	}

	err = playSound(c, toChannel, SoundConfig{SoundID: "1449829431693672641", Duration: 1 * time.Second})
	if err != nil {
		return moved, fmt.Errorf("failed to play leaving sound: %v", err)
	}

	return moved, nil
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
