package internal

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/robfig/cron/v3"
)

var (
	logger = NewLogger("github.com/robert-mccausland/janitor-bot/internal/discord")
)

func Janate(s *discordgo.Session) error {
	officeChannelId := os.Getenv("OFFICE_CHANNEL_ID")
	office, err := s.Channel(officeChannelId)
	if err != nil {
		return fmt.Errorf("failed to get office channel, %v", slog.Any("err", err))
	}

	defaultChannelId := os.Getenv("DEFAULT_CHANNEL_ID")
	defaultChannel, err := s.Channel(defaultChannelId)
	if err != nil {
		return fmt.Errorf("failed to get default channel, %v", slog.Any("err", err))
	}

	timezoneName := os.Getenv("TIMEZONE")
	timezone, err := time.LoadLocation(timezoneName)
	if err != nil {
		return fmt.Errorf("invalid timezone provided: %s", timezoneName)
	}

	cron := cron.New(cron.WithLocation(timezone))

	_, err = cron.AddFunc("00 17 * * *", func() {
		logger.Info("Closing the office")
		err := closeOffice(s, office, defaultChannel)
		if err != nil {
			logger.Error("error while closing the office: %v", slog.Any("err", err))
		}
	})
	if err != nil {
		return err
	}

	_, err = cron.AddFunc("00 09 * * *", func() {
		logger.Info("Opening the office")
		err := openOffice(s, office)
		if err != nil {
			logger.Error("error while opening the office: %v", slog.Any("err", err))
		}
	})
	if err != nil {
		return err
	}

	cron.Start()

	return nil
}

func openOffice(s *discordgo.Session, office *discordgo.Channel) error {
	err := s.ChannelPermissionDelete(office.ID, office.GuildID)
	if err != nil {
		return fmt.Errorf("failed to edit permisions to open office: %v", err)
	}

	return nil
}

func closeOffice(s *discordgo.Session, office *discordgo.Channel, defaultChannel *discordgo.Channel) error {

	// Its important to give the janitor spesific permissions to join the channel, as tje janitor will be unable to grant it
	// again if it doesn't have the permission and removing the join permission from the everyone group will do this.
	err := s.ChannelPermissionSet(office.ID, s.State.User.ID, discordgo.PermissionOverwriteTypeMember, discordgo.PermissionVoiceConnect, 0)
	if err != nil {
		return fmt.Errorf("failed to add janitor exception permission to office: %v", err)
	}
	err = s.ChannelPermissionSet(office.ID, office.GuildID, discordgo.PermissionOverwriteTypeRole, 0, discordgo.PermissionVoiceConnect)
	if err != nil {
		return fmt.Errorf("failed to remove connect permissions to office: %v", err)
	}

	guild, err := s.State.Guild(office.GuildID)
	var inOffice []*discordgo.VoiceState
	for _, vs := range guild.VoiceStates {
		if vs.ChannelID == office.ID {
			inOffice = append(inOffice, vs)
		}
	}

	if len(inOffice) == 0 {
		return nil
	}

	vc, err := s.ChannelVoiceJoin(office.GuildID, office.ID, false, false)
	if err != nil {
		return fmt.Errorf("failed to join voice channel: %v", err)

	}

	// VC connection is not always properly established so wait a little bit
	time.Sleep(200 * time.Millisecond)

	defer func() {
		err = vc.Disconnect()
		if err != nil {
			logger.Error("Failed to disconnect from voice: %v", slog.Any("err", err))
		}
	}()

	err = playSound(s, office, SoundConfig{SoundID: "1223777210650067056", Duration: 2 * time.Second})
	if err != nil {
		return fmt.Errorf("failed to play joining sound: %v", err)
	}

	for _, vs := range inOffice {
		err := s.GuildMemberMove(office.GuildID, vs.UserID, &defaultChannel.ID)
		if err != nil {
			return fmt.Errorf("error moving user %s: %v", vs.UserID, err)
		}
	}

	err = vc.ChangeChannel(defaultChannel.ID, false, false)
	if err != nil {
		return fmt.Errorf("failed to switch channels: %v", err)
	}

	// VC connection is not always properly established so wait a little bit
	time.Sleep(200 * time.Millisecond)

	err = playSound(s, office, SoundConfig{SoundID: "1449829431693672641", Duration: 1 * time.Second})
	if err != nil {
		return fmt.Errorf("failed to play leaving sound: %v", err)

	}

	return nil
}

type SoundConfig struct {
	SoundID  string
	Duration time.Duration
}

func playSound(s *discordgo.Session, channel *discordgo.Channel, data SoundConfig) error {
	err := SendSoundboardSound(s, channel.ID, SoundboardSoundSend{SoundID: data.SoundID, GuildID: channel.GuildID})
	if err != nil {
		return err
	}
	time.Sleep(data.Duration)
	return nil
}
