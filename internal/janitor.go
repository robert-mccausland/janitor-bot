package internal

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"time"

	"github.com/robert-mccausland/janitor-bot/internal/discord"
	"github.com/robert-mccausland/janitor-bot/internal/logging"
	"github.com/robfig/cron/v3"
)

var logger *slog.Logger

func init() {
	logger = logging.NewLogger("github.com/robert-mccausland/janitor-bot/internal")
}

func Janate(c *discord.Client, ctx context.Context) error {
	officeChannelId := os.Getenv("OFFICE_CHANNEL_ID")
	office, err := c.GetChannel(officeChannelId)
	if err != nil {
		return fmt.Errorf("failed to get office channel, %v", slog.Any("err", err))
	}

	defaultChannelId := os.Getenv("DEFAULT_CHANNEL_ID")
	defaultChannel, err := c.GetChannel(defaultChannelId)
	if err != nil {
		return fmt.Errorf("failed to get default channel, %v", slog.Any("err", err))
	}

	timezoneName := os.Getenv("TIMEZONE")
	timezone, err := time.LoadLocation(timezoneName)
	if err != nil {
		return fmt.Errorf("invalid timezone provided: %s", timezoneName)
	}

	cron := cron.New(cron.WithLocation(timezone))

	_, err = cron.AddFunc("00 17 * * 1-5", func() {
		logger.Info("Closing the office")
		err := closeOffice(c, office, defaultChannel)
		if err != nil {
			logger.Error("error while closing the office: %v", slog.Any("err", err))
		}
	})
	if err != nil {
		return err
	}

	_, err = cron.AddFunc("00 09 * * 1-5", func() {
		logger.Info("Opening the office")
		err := openOffice(c, office)
		if err != nil {
			logger.Error("error while opening the office: %v", slog.Any("err", err))
		}
	})
	if err != nil {
		return err
	}

	go func() {
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
		case <-ctx.Done():
			return
		}

		for {
			channel := office
			if rand.Intn(2) == 0 {
				channel = defaultChannel
			}
			err := sweepChannel(c, channel)
			if err != nil {
				logger.Error(fmt.Sprintf("error while sweeping channel: %v", err), slog.Any("err", err))
			}

			select {
			case <-time.After(getTimeout()):
			case <-ctx.Done():
				return
			}
		}
	}()

	cron.Start()

	return nil
}

func sweepChannel(c *discord.Client, channel *discord.Channel) error {
	logger.Info("Sweeping channel", slog.String("channel_id", channel.ID))

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

func openOffice(c *discord.Client, office *discord.Channel) error {
	logger.Info("Opening office", slog.String("channel_id", office.ID))
	err := c.DeleteChannelPermissions(office.ID, office.GuildID)
	if err != nil {
		return fmt.Errorf("failed to edit permission to open office: %w", err)
	}

	return nil
}

func closeOffice(c *discord.Client, office *discord.Channel, defaultChannel *discord.Channel) error {
	logger.Info("Closing office", slog.String("channel_id", office.ID), slog.String("default_channel_id", defaultChannel.ID))

	// Its important to give the janitor specific permissions to join the channel, as the janitor will be unable to grant it
	// again if it doesn't have the permission and removing the join permission from the everyone group will do this.
	err := c.EditChannelPermissions(office.ID, discord.PermissionOverwrite{
		ID:    c.User().ID,
		Type:  discord.PermissionOverwriteTypeMember,
		Allow: discord.PermissionConnect,
		Deny:  discord.PermissionNone,
	})
	if err != nil {
		return fmt.Errorf("failed to add janitor exception permission to office: %v", err)
	}

	err = c.EditChannelPermissions(office.ID, discord.PermissionOverwrite{
		ID:    office.GuildID,
		Type:  discord.PermissionOverwriteTypeRole,
		Allow: discord.PermissionNone,
		Deny:  discord.PermissionConnect,
	})
	if err != nil {
		return fmt.Errorf("failed to remove connect permissions to office: %v", err)
	}

	guild := c.Guild(office.GuildID)
	if guild == nil {
		return fmt.Errorf("office guild not found: %s", office.GuildID)
	}

	var inOffice []discord.VoiceState
	for _, vs := range guild.VoiceStates {
		if vs.ChannelID != nil && *vs.ChannelID == office.ID {
			inOffice = append(inOffice, vs)
		}
	}

	if len(inOffice) == 0 {
		return nil
	}

	logger.Info("Moving people out of the office office", slog.String("channel_id", office.ID), slog.String("default_channel_id", defaultChannel.ID))
	vc, err := c.JoinVoiceChannel(office.GuildID, office.ID, false, false)
	if err != nil {
		return fmt.Errorf("failed to join voice channel: %v", err)
	}

	defer func() {
		err = vc.Leave()
		if err != nil {
			logger.Error("Failed to disconnect from voice: %v", slog.Any("err", err))
		}
	}()

	err = playSound(c, office, SoundConfig{SoundID: "1223777210650067056", Duration: 2 * time.Second})
	if err != nil {
		return fmt.Errorf("failed to play joining sound: %w", err)
	}

	for _, vs := range inOffice {
		err := c.ModifyGuildMember(office.GuildID, vs.UserID, discord.GuildMemberUpdate{
			ChannelID: &defaultChannel.ID,
		})
		if err != nil {
			return fmt.Errorf("error moving user %s: %v", vs.UserID, err)
		}
	}

	err = vc.ChangeChannel(defaultChannel.ID, false, false)
	if err != nil {
		return fmt.Errorf("failed to switch channels: %v", err)
	}

	err = playSound(c, defaultChannel, SoundConfig{SoundID: "1449829431693672641", Duration: 1 * time.Second})
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
	err := c.SendSoundboardSound(channel.ID, discord.SendSoundboardSound{SoundID: data.SoundID})
	if err != nil {
		return err
	}
	time.Sleep(data.Duration)
	return nil
}
