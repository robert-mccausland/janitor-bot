package janitor

import (
	"fmt"
	"log/slog"
	"math/rand"
	"time"

	"github.com/robert-mccausland/janitor-bot/internal/discord"
)

func (j *Janitor) runChannelSweepLoop() {
	getTimeout := func() time.Duration {
		minSweepInterval := 3 * time.Hour
		maxSweepInterval := 9 * time.Hour
		jitter := time.Duration(rand.Int63n(int64(maxSweepInterval - minSweepInterval)))
		timeout := minSweepInterval + jitter
		return timeout
	}

	timeout := time.Duration(rand.Int63n(int64(getTimeout())))
	select {
	case <-time.After(timeout):
	case <-j.ctx.Done():
		return
	}

	for {
		err := sweepChannel(j.client, j.defaultChannel)
		if err != nil {
			logger.Error(fmt.Sprintf("error while sweeping channel: %v", err), slog.Any("err", err))
		}

		select {
		case <-time.After(getTimeout()):
		case <-j.ctx.Done():
			return
		}
	}
}

func sweepChannel(c *discord.Client, defaultChannel *discord.Channel) error {
	logger.Info("Sweeping channel", slog.String("default_channel_id", defaultChannel.ID))

	states := c.Guild(defaultChannel.GuildID).VoiceStates
	vcMap := map[string]struct{}{}
	for id := range states {
		channelID := states[id].ChannelID
		if channelID != nil {
			vcMap[*channelID] = struct{}{}
		}
	}

	var voiceChannels []string
	if len(vcMap) == 0 {
		channels, err := c.GetChannels(defaultChannel.GuildID)
		if err != nil {
			return fmt.Errorf("failed to get channels: %v", err)
		}

		for _, c := range *channels {
			if c.Type == discord.ChannelTypeGuildVoice {
				voiceChannels = append(voiceChannels, c.ID)
			}
		}
	} else {
		voiceChannels = make([]string, 0, len(vcMap))
		for id := range vcMap {
			voiceChannels = append(voiceChannels, id)
		}
	}

	index := rand.Intn(len(voiceChannels))
	channel, err := c.GetChannel(voiceChannels[index])
	if err != nil {
		return fmt.Errorf("failed to get channel: %v", err)
	}

	vc, err := c.JoinVoiceChannel(channel.GuildID, channel.ID, false, false)
	if err != nil {
		return fmt.Errorf("failed to join voice channel: %v", err)
	}
	defer func() {
		err = vc.Leave()
		if err != nil {
			logger.Error(fmt.Sprintf("Failed to disconnect from voice: %v", err), slog.Any("error", err))
		}
	}()

	<-time.After(time.Second * 1)

	sounds, err := c.GetSoundboardSounds(channel.GuildID)
	if err != nil {
		return fmt.Errorf("failed to find soundboard sounds: %v", err)
	}

	if len(*sounds) != 0 {
		chosenSound := (*sounds)[rand.Intn(len(*sounds))]
		logger.Info("Fling it now!")
		err = playSound(c, channel, SoundConfig{SoundID: chosenSound.SoundID, Duration: 1 * time.Second})
		if err != nil {
			return fmt.Errorf("failed to play sweep sound: %w", err)
		}
	}

	<-time.After(time.Second * time.Duration(rand.Intn(7)+3))
	return nil
}
