package internal

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"sync"
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

	repositoryOptions := RepositoryOptions{
		dbPath: os.Getenv("DB_PATH"),
	}
	repo, err := NewRepository(repositoryOptions)
	if err != nil {
		return fmt.Errorf("failed to create repository: %v", err)
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
		START_HOUR := 6
		// Avoid running very close to midnight as delays could cause the run to finish after midnight.
		END_BUFFER_SECONDS := 30

		lastRun, err := repo.GetLastMessageSent()
		if err != nil {
			logger.Error("Error while fetching last message sent: %v", slog.Any("err", err))
		}

		for {
			runStart := time.Now().In(timezone)
			var windowStart time.Time
			if lastRun == nil {
				windowStart = time.Date(runStart.Year(), runStart.Month(), runStart.Day(), START_HOUR, 0, 0, 0, timezone)
			} else {
				windowStart = time.Date(lastRun.Year(), lastRun.Month(), lastRun.Day(), START_HOUR, 0, 0, 0, timezone).AddDate(0, 0, 1)
			}

			nextDayStart := time.Date(windowStart.Year(), windowStart.Month(), windowStart.Day(), 0, 0, 0, 0, timezone).AddDate(0, 0, 1)
			windowEnd := nextDayStart.Add(-time.Duration(END_BUFFER_SECONDS) * time.Second)

			logger.Info(fmt.Sprintf("Preparing plank's reckoning... Window Start: %v, Window End: %v\n", windowStart, windowEnd),
				slog.Time("window_start", windowStart),
				slog.Time("window_end", windowEnd),
			)

			waitTime := time.Until(windowStart)
			if waitTime > 0 {
				logger.Info(fmt.Sprint("Waiting for window start at: ", windowStart))
				select {
				case <-time.After(waitTime):
				case <-ctx.Done():
					return
				}
			}

			window := time.Until(windowEnd)
			if window <= 0 {
				logger.Info("Looks like the window is already closed, skipping for today",
					slog.Time("window_start", windowStart),
					slog.Time("window_end", windowEnd),
				)

				// Set this to the end of the window so that we don't try to run again until the next day to avoid a tight loop.
				lastRun = &windowEnd
				continue
			}

			timeout := time.Duration(rand.Int63n(int64(window)))
			logger.Info(fmt.Sprintf("Waiting for %v before doing planks reckoning", timeout), slog.Duration("timeout", timeout))
			select {
			case <-time.After(timeout):
			case <-ctx.Done():
				return
			}

			if err := planksReckoning(c); err != nil {
				logger.Error("error while doing planks reckoning", slog.Any("err", err))
			}

			now := time.Now().In(timezone)
			lastRun = &now
			if err := repo.SetLastMessageSent(now); err != nil {
				logger.Error("Error while setting last message sent: %v", slog.Any("err", err))
			}
		}
	}()

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
			err := sweepChannel(c, defaultChannel)
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

	mu := sync.Mutex{}
	c.On(discord.EventVoiceStateUpdate, func() error {
		if mu.TryLock() {
			go func() {
				err := checkChannels(c, office, timezone)
				if err != nil {
					logger.Error(fmt.Sprintf("error while checking channels: %v", err), slog.Any("err", err))
				}
				defer mu.Unlock()
			}()
		}
		return nil
	})

	cron.Start()

	return nil
}

func planksReckoning(c *discord.Client) error {
	logger.Info("Planks reckoning")

	arsenalChampionsDate, err := time.Parse("2006-01-02", os.Getenv("ARSENAL_CHAMPIONS_DATE"))
	if err != nil {
		return fmt.Errorf("invalid ARSENAL_CHAMPIONS_DATE environment variable: %v", err)
	}
	planksId := os.Getenv("PLANKS_ID")
	channelId := os.Getenv("FOOTBALL_CHANNEL_ID")

	daysSinceChampions := int(time.Since(arsenalChampionsDate).Hours() / 24)
	celebratoryMessage := fmt.Sprintf("Daily update: <@%s> it has been %d days since ARSENAL became CHAMPIONS OF ENGLAND.", planksId, daysSinceChampions)

	err = c.CreateMessage(channelId, discord.Message{Content: celebratoryMessage})
	if err != nil {
		return fmt.Errorf("failed to post celebratory message: %v", err)
	}

	return nil
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

	err = moveUsers(c, office, defaultChannel)
	if err != nil {
		return fmt.Errorf("failed to move users from office to default channel: %v", err)
	}

	return nil
}

func checkChannels(c *discord.Client, office *discord.Channel, timezone *time.Location) error {
	logger.Info("Checking channels to see if current users are in the correct place")

	now := time.Now().In(timezone)
	weekday := now.Weekday()
	if weekday < time.Monday || weekday > time.Friday {
		return nil
	}

	start := time.Date(now.Year(), now.Month(), now.Day(), 9, 0, 0, 0, timezone)
	end := time.Date(now.Year(), now.Month(), now.Day(), 16, 30, 0, 0, timezone)
	if now.Before(start) || now.After(end) {
		return nil
	}

	// Wait for 5 to 10 minutes before moving anyone.
	waitTime := time.Duration(rand.Intn(5)+5) * time.Minute
	logger.Info(fmt.Sprintf("Its the correct time to check channels, waiting for %v before moving users.", waitTime))
	time.Sleep(waitTime)

	channels, err := c.GetChannels(office.GuildID)
	if err != nil {
		return fmt.Errorf("failed to get channels: %v", err)
	}

	for _, channel := range *channels {
		if channel.Type != discord.ChannelTypeGuildVoice {
			continue
		}

		if channel.ID == office.ID {
			continue
		}

		err = moveUsers(c, &channel, office)
		if err != nil {
			return fmt.Errorf("failed to move users from %s to office: %v", channel.ID, err)
		}
	}

	return nil
}

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
	err := c.SendSoundboardSound(channel.ID, discord.SendSoundboardSound{SoundID: data.SoundID})
	if err != nil {
		return err
	}
	time.Sleep(data.Duration)
	return nil
}
