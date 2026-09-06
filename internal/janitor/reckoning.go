package janitor

import (
	"fmt"
	"log/slog"
	"math/rand"
	"time"

	"github.com/robert-mccausland/janitor-bot/internal/discord"
)

func (j *Janitor) runPlanksReckoningLoop() {
	START_HOUR := 6
	// Avoid running very close to midnight as delays could cause the run to finish after midnight.
	END_BUFFER_SECONDS := 30

	lastRun, err := j.repo.GetLastMessageSent()
	if err != nil {
		logger.Error("Error while fetching last message sent", slog.Any("err", err))
	}

	for {
		runStart := time.Now().In(j.timezone)
		var windowStart time.Time
		if lastRun == nil {
			windowStart = time.Date(runStart.Year(), runStart.Month(), runStart.Day(), START_HOUR, 0, 0, 0, j.timezone)
		} else {
			windowStart = time.Date(lastRun.Year(), lastRun.Month(), lastRun.Day(), START_HOUR, 0, 0, 0, j.timezone).AddDate(0, 0, 1)
		}

		nextDayStart := time.Date(windowStart.Year(), windowStart.Month(), windowStart.Day(), 0, 0, 0, 0, j.timezone).AddDate(0, 0, 1)
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
			case <-j.ctx.Done():
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
		case <-j.ctx.Done():
			return
		}

		if err := j.planksReckoning(); err != nil {
			logger.Error("error while doing planks reckoning", slog.Any("err", err))
		}

		now := time.Now().In(j.timezone)
		lastRun = &now
		if err := j.repo.SetLastMessageSent(now); err != nil {
			logger.Error("Error while setting last message sent", slog.Any("err", err))
		}
	}
}

func (j *Janitor) planksReckoning() error {
	logger.Info("Planks reckoning")

	daysSinceChampions := int(time.Since(j.arsenalChampionsDate).Hours() / 24)
	celebratoryMessage := fmt.Sprintf("Daily update: <@%s> it has been %d days since ARSENAL became CHAMPIONS OF ENGLAND.", j.planksID, daysSinceChampions)

	err := j.client.CreateMessage(j.footballChannelID, discord.Message{Content: celebratoryMessage})
	if err != nil {
		return fmt.Errorf("failed to post celebratory message: %v", err)
	}

	return nil
}
