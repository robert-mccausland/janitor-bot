package janitor

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/robert-mccausland/janitor-bot/internal"
	"github.com/robert-mccausland/janitor-bot/internal/discord"
	"github.com/robert-mccausland/janitor-bot/internal/logging"
	"github.com/robfig/cron/v3"
)

var logger *slog.Logger

func init() {
	logger = logging.NewLogger("github.com/robert-mccausland/janitor-bot/internal/janitor")
}

type Janitor struct {
	client         *discord.Client
	office         *discord.Channel
	defaultChannel *discord.Channel
	holidays       *internal.Holidays
	repo           *internal.Repository
	ctx            context.Context

	officeChannelID      string
	defaultChannelID     string
	timezone             *time.Location
	dbPath               string
	arsenalChampionsDate time.Time
	planksID             string
	footballChannelID    string
}

func NewJanitor(ctx context.Context, config internal.Config) (*Janitor, error) {

	logger.Info("Loading holiday information")
	holidays := internal.NewHolidays(internal.DefaultHolidaysConfig())
	if err := holidays.Refresh(ctx); err != nil {
		return nil, fmt.Errorf("failed to refresh holidays: %v", err)
	}

	logger.Info("Creating discord client")
	client, err := SetupDiscordClient(ctx, config.DiscordToken)
	if err != nil {
		return nil, fmt.Errorf("failed to setup discord client: %v", err)
	}

	office, err := client.GetChannel(config.OfficeChannelID)
	if err != nil {
		return nil, fmt.Errorf("failed to get office channel: %v", err)
	}

	defaultChannel, err := client.GetChannel(config.DefaultChannelID)
	if err != nil {
		return nil, fmt.Errorf("failed to get default channel: %v", err)
	}

	logger.Info("Creating repository")
	repo, err := internal.NewRepository(internal.RepositoryOptions{DBPath: config.DBPath})
	if err != nil {
		return nil, fmt.Errorf("failed to create repository: %v", err)
	}

	logger.Info("Janitor created successfully")
	return &Janitor{
		client:               client,
		office:               office,
		defaultChannel:       defaultChannel,
		holidays:             holidays,
		repo:                 repo,
		ctx:                  ctx,
		officeChannelID:      config.OfficeChannelID,
		defaultChannelID:     config.DefaultChannelID,
		timezone:             config.Timezone,
		dbPath:               config.DBPath,
		arsenalChampionsDate: config.ArsenalChampionsDate,
		planksID:             config.PlanksID,
		footballChannelID:    config.FootballChannelID,
	}, nil
}

func (j *Janitor) Janate() error {
	logger.Info("Setting up cron jobs")
	officeCron := cron.New(cron.WithLocation(j.timezone))
	if _, err := officeCron.AddFunc("00 09 * * 1-5", func() { j.setScheduledOfficeState(true) }); err != nil {
		return err
	}
	if _, err := officeCron.AddFunc("00 17 * * 1-5", func() { j.setScheduledOfficeState(false) }); err != nil {
		return err
	}

	logger.Info("Checking office state")
	j.reconcileOfficeState()

	logger.Info("Creating background loops")
	go j.runPlanksReckoningLoop()
	go j.runChannelSweepLoop()
	j.registerChannelChecker()

	officeCron.Start()

	logger.Info("janitor-bot started successfully")

	return j.client.BlockUntilDone()
}

func SetupDiscordClient(ctx context.Context, token string) (*discord.Client, error) {
	client := discord.NewDiscordClient(discord.DefaultOptions())
	err := client.Start(ctx, token, discord.IntentGuilds|discord.IntentGuildVoiceStates)
	if err != nil {
		return nil, fmt.Errorf("error starting discord client: %w", err)
	}

	return client, nil
}
