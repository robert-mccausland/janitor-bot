package janitor

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/robert-mccausland/janitor-bot/internal"
	"github.com/robert-mccausland/janitor-bot/internal/discord"
)

func (j *Janitor) applyOfficeState(open bool) {
	var err error
	if open {
		err = openOffice(j.client, j.office)
	} else {
		err = closeOffice(j.client, j.office, j.defaultChannel)
	}

	if err != nil {
		action := "opening"
		if !open {
			action = "closing"
		}
		logger.Error(fmt.Sprintf("error while %s the office: %v", action, err), slog.Any("err", err), slog.String("action", action))
	}
}

func (j *Janitor) setScheduledOfficeState(open bool) {
	holiday, err := j.holidays.GetHoliday(j.ctx, time.Now().In(j.timezone))
	if err != nil {
		logger.Error("error while checking for holiday", slog.Any("err", err))
	}

	if holiday != nil {
		logger.Info(fmt.Sprintf("Today is a holiday: %s, skipping updating the office", holiday.Title), slog.String("holiday_title", holiday.Title))
		return
	}

	j.applyOfficeState(open)
}

func (j *Janitor) reconcileOfficeState() {
	shouldBeOpen, err := officeShouldBeOpenNow(j.ctx, j.holidays, j.timezone)
	if err != nil {
		logger.Error("error determining office state on startup", slog.Any("err", err))
		return
	}

	logger.Info("Reconciling office state on startup", slog.Bool("should_be_open", shouldBeOpen))
	j.applyOfficeState(shouldBeOpen)
}

func officeShouldBeOpenNow(ctx context.Context, holidays *internal.Holidays, timezone *time.Location) (bool, error) {
	now := time.Now().In(timezone)
	if now.Weekday() < time.Monday || now.Weekday() > time.Friday {
		return false, nil
	}

	holiday, err := holidays.GetHoliday(ctx, now)
	if err != nil {
		return false, err
	}
	if holiday != nil {
		return false, nil
	}

	open := time.Date(now.Year(), now.Month(), now.Day(), 9, 0, 0, 0, timezone)
	close := time.Date(now.Year(), now.Month(), now.Day(), 17, 0, 0, 0, timezone)
	return !now.Before(open) && now.Before(close), nil
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
