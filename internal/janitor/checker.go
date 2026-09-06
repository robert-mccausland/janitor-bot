package janitor

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"sync"
	"time"

	"github.com/robert-mccausland/janitor-bot/internal"
	"github.com/robert-mccausland/janitor-bot/internal/discord"
)

func (j *Janitor) registerChannelChecker() {
	mu := sync.Mutex{}
	j.client.On(discord.EventVoiceStateUpdate, func() error {
		if mu.TryLock() {
			go func() {
				defer mu.Unlock()
				err := checkChannels(j.client, j.office, j.holidays, j.timezone, j.ctx)
				if err != nil {
					logger.Error(fmt.Sprintf("error while checking channels: %v", err), slog.Any("err", err))
				}
			}()
		}
		return nil
	})
}

func checkChannels(c *discord.Client, office *discord.Channel, holidays *internal.Holidays, timezone *time.Location, ctx context.Context) error {
	logger.Info("Checking channels to see if current users are in the correct place")

	now := time.Now().In(timezone)
	weekday := now.Weekday()
	if weekday < time.Monday || weekday > time.Friday {
		return nil
	}

	holiday, err := holidays.GetHoliday(ctx, now)

	if err != nil {
		logger.Error("error while checking for holiday: %v", slog.Any("err", err))
	}
	if holiday != nil {
		logger.Info(fmt.Sprintf("Today is a holiday: %s, skipping checking channels", holiday.Title), slog.String("holiday_title", holiday.Title))
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
