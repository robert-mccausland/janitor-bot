package internal

import (
	"errors"
	"fmt"
	"os"
	"time"
)

type Config struct {
	OfficeChannelID      string
	DefaultChannelID     string
	Timezone             *time.Location
	DBPath               string
	ArsenalChampionsDate time.Time
	PlanksID             string
	FootballChannelID    string
	DiscordToken         string

	errs []error
}

func getEnv(c *Config, key string) string {
	value := os.Getenv(key)
	if value == "" {
		c.errs = append(c.errs, fmt.Errorf("no %s environment variable found", key))
	}
	return value
}

func getEnvParsed[T any](c *Config, key string, parse func(string) (T, error)) T {
	value := getEnv(c, key)
	if value == "" {
		return *new(T)
	}

	parsedValue, err := parse(value)
	if err != nil {
		c.errs = append(c.errs, fmt.Errorf("failed to parse %s environment variable '%s': %w", key, value, err))
		return *new(T)
	}
	return parsedValue
}

func GetConfig() (*Config, error) {
	c := &Config{}

	c.OfficeChannelID = getEnv(c, "OFFICE_CHANNEL_ID")
	c.DefaultChannelID = getEnv(c, "DEFAULT_CHANNEL_ID")
	c.Timezone = getEnvParsed(c, "TIMEZONE", time.LoadLocation)
	c.DBPath = getEnv(c, "DB_PATH")
	c.ArsenalChampionsDate = getEnvParsed(c, "ARSENAL_CHAMPIONS_DATE", func(s string) (time.Time, error) {
		time, err := time.Parse("2006-01-02", s)
		return time, err
	})
	c.PlanksID = getEnv(c, "PLANKS_ID")
	c.FootballChannelID = getEnv(c, "FOOTBALL_CHANNEL_ID")
	c.DiscordToken = getEnv(c, "TOKEN")

	if len(c.errs) > 0 {
		return nil, errors.Join(c.errs...)
	}

	return c, nil
}
