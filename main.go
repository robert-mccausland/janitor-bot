package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
	"github.com/robfig/cron/v3"

	"github.com/robert-mccausland/janitor-bot/internal"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		fmt.Println("Warning: could not load .env file", err)
	}
	token := os.Getenv("TOKEN")

	s, err := discordgo.New("Bot " + token)
	if err != nil {
		fmt.Println("Error creating Discord session,", err)
		return
	}

	ready := make(chan bool)
	s.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		fmt.Println("Janitor bot is ready")
		ready <- true
	})

	s.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildVoiceStates
	err = s.Open()
	if err != nil {
		fmt.Println("Error opening connection,", err)
		return
	}

	<-ready
	err = janate(s)
	if err != nil {
		fmt.Println("Error while janitoring: ", err)
		return
	}

	fmt.Println("janitor-bot has started successfully!")
	wait := make(chan os.Signal, 1)
	signal.Notify(wait, syscall.SIGINT, syscall.SIGTERM)
	<-wait

	err = s.Close()
	if err != nil {
		fmt.Println("Error while closing discord connection: ", err)
		return
	}
}

func janate(s *discordgo.Session) error {
	officeChannelId := os.Getenv("OFFICE_CHANNEL_ID")
	office, err := s.Channel(officeChannelId)
	if err != nil {
		return fmt.Errorf("failed to get office channel, %v", err)
	}

	defaultChannelId := os.Getenv("DEFAULT_CHANNEL_ID")
	defaultChannel, err := s.Channel(defaultChannelId)
	if err != nil {
		return fmt.Errorf("failed to get default channel, %v", err)
	}

	timezoneName := os.Getenv("TIMEZONE")
	timezone, err := time.LoadLocation(timezoneName)
	if err != nil {
		return fmt.Errorf("invalid timezone provided: %s", timezoneName)
	}

	cron := cron.New(cron.WithLocation(timezone))

	_, err = cron.AddFunc("00 17 * * *", func() {
		fmt.Println("Close the office")
		err := closeOffice(s, office, defaultChannel)
		if err != nil {
			println(err)
		}
	})
	if err != nil {
		return err
	}

	_, err = cron.AddFunc("00 09 * * *", func() {
		fmt.Println("Open the office")
		err := openOffice(s, office)
		if err != nil {
			println(err)
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

	defer func() {
		err = vc.Disconnect()
		if err != nil {
			fmt.Printf("Failed to disconnect from voice: %v", err)
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
	err := internal.SendSoundboardSound(s, channel.ID, internal.SoundboardSoundSend{SoundID: data.SoundID, GuildID: channel.GuildID})
	if err != nil {
		return err
	}
	time.Sleep(data.Duration)
	return nil
}
