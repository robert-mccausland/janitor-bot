package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type division string

const (
	EnglandAndWales division = "england-and-wales"
	Scotland        division = "scotland"
	NorthernIreland division = "northern-ireland"
)

type HolidaysConfig struct {
	URL      string
	TTL      time.Duration
	Division division
}

type Holidays struct {
	config    HolidaysConfig
	mu        sync.Mutex
	holidays  holidays
	fetchedAt time.Time
	client    *http.Client
}

type divisionFeed struct {
	Division division `json:"division"`
	Events   []event  `json:"events"`
}

type event struct {
	Title string `json:"title"`
	Date  string `json:"date"`
	Notes string `json:"notes"`
}

type feed map[division]divisionFeed

type holidays = map[string]Holiday

type Holiday = struct {
	Title string
	Notes string
}

func DefaultHolidaysConfig() HolidaysConfig {
	return HolidaysConfig{
		URL:      "https://www.gov.uk/bank-holidays.json",
		TTL:      24 * time.Hour,
		Division: EnglandAndWales,
	}
}

func NewHolidays(config HolidaysConfig) *Holidays {
	return &Holidays{
		config: config,
		mu:     sync.Mutex{},
	}
}

func (c *Holidays) Refresh(ctx context.Context) error {
	return c.load(ctx, true)
}

func (c *Holidays) GetHoliday(ctx context.Context, time time.Time) (*Holiday, error) {
	err := c.load(ctx, false)
	if err != nil {
		return nil, err
	}
	date := time.Format("2006-01-02")

	c.mu.Lock()
	defer c.mu.Unlock()
	holiday, ok := c.holidays[date]
	if !ok {
		return nil, nil
	}

	return &holiday, nil
}

func (c *Holidays) load(ctx context.Context, force bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.holidays != nil && time.Since(c.fetchedAt) < c.config.TTL && !force {
		return nil
	}

	holidays, err := c.fetch(ctx)
	if err != nil {
		return err
	}

	c.holidays, c.fetchedAt = holidays, time.Now()
	return nil
}

func (c *Holidays) fetch(ctx context.Context) (holidays, error) {
	url := c.config.URL
	client := c.client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching bank holidays: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching bank holidays: unexpected status %s", resp.Status)
	}

	var feed feed
	if err := json.NewDecoder(resp.Body).Decode(&feed); err != nil {
		return nil, fmt.Errorf("decoding bank holidays: %w", err)
	}

	div, ok := feed[c.config.Division]
	if !ok {
		return nil, fmt.Errorf("no such division %q in feed", c.config.Division)
	}
	if len(div.Events) == 0 {
		return nil, fmt.Errorf("division %q has no events", c.config.Division)
	}

	var holidays = make(map[string]Holiday)
	for _, e := range div.Events {
		holidays[e.Date] = Holiday{Title: e.Title, Notes: e.Notes}
	}

	return holidays, nil
}
