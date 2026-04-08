package discord

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/robert-mccausland/janitor-bot/internal/logging"
)

var (
	logger = logging.NewLogger("github.com/robert-mccausland/janitor-bot/internal/discord")
)

type Client struct {
	token   string
	intents int
	options ClientOptions

	ctx        context.Context
	httpClient *http.Client
	isRunning  bool

	gatewayURL         *string
	user               *userObject
	sessionID          *string
	resumeURL          *string
	lastSequenceNumber atomic.Int64
	wc                 *websocketClient

	loadingCh chan struct{}
	errorCh   chan error

	mu                sync.RWMutex
	guilds            map[string]Guild
	unavailableGuilds map[string]struct{}

	voiceMu           sync.RWMutex
	voiceJoinSessions map[string]voiceUpdateSession
}

type ClientOptions struct {
	ApiURL                    string
	ClientIdentifier          string
	HeartbeatMaxDelay         time.Duration
	WSReadyTimeout            time.Duration
	VoiceJoinTimeout          time.Duration
	RestAPITimeout            time.Duration
	HttpIdleConnectionTimeout time.Duration
	HttpMaxIdleConnections    int
	WebsocketRetryTimeout     time.Duration
	WebsocketRetryLimit       int
	StatusLogInterval         time.Duration
}

func DefaultOptions() ClientOptions {
	return ClientOptions{
		ApiURL:                    "https://discord.com/api/v10",
		ClientIdentifier:          "janitor-bot-client",
		HeartbeatMaxDelay:         5 * time.Second,
		WSReadyTimeout:            10 * time.Second,
		VoiceJoinTimeout:          10 * time.Second,
		RestAPITimeout:            10 * time.Second,
		HttpIdleConnectionTimeout: 60 * time.Second,
		HttpMaxIdleConnections:    10,
		WebsocketRetryTimeout:     1 * time.Second,
		WebsocketRetryLimit:       5,
		StatusLogInterval:         15 * time.Minute,
	}
}

type Guild struct {
	ID          string
	Name        string
	Channels    []Channel
	VoiceStates map[string]VoiceState
}

type VoiceState struct {
	ChannelID *string
	UserID    string
	SessionID string
}

type Channel struct {
	ID                   string
	Type                 ChannelType
	GuildID              string
	Position             int
	PermissionOverwrites []PermissionOverwrite
	Name                 *string
}

type User struct {
	ID            string
	Username      string
	Discriminator string
	GlobalName    string
	Avatar        string
	Bot           bool
}

type PermissionOverwrite struct {
	ID    string
	Type  PermissionOverwriteType
	Allow Permission
	Deny  Permission
}

func NewDiscordClient(options ClientOptions) *Client {
	client := Client{
		options:           options,
		loadingCh:         make(chan struct{}),
		unavailableGuilds: make(map[string]struct{}),
		guilds:            make(map[string]Guild),
		voiceJoinSessions: make(map[string]voiceUpdateSession),
		httpClient: &http.Client{
			Timeout: options.RestAPITimeout,
			Transport: &http.Transport{
				MaxIdleConns:    options.HttpMaxIdleConnections,
				IdleConnTimeout: options.HttpIdleConnectionTimeout,
			},
		},
	}

	return &client
}

func (d *Client) Start(ctx context.Context, token string, intents Intent) error {
	if d.isRunning {
		return fmt.Errorf("discord client is already running")
	}

	d.isRunning = true
	d.token = token
	d.intents = int(intents)
	d.ctx = ctx

	gatewayURL, err := d.getGatewayURL()
	if err != nil {
		return fmt.Errorf("unable to get gateway URL: %w", err)
	}

	d.gatewayURL = &gatewayURL

	err = d.startWebsockets()
	if err != nil {
		return fmt.Errorf("error while starting websocket client: %w", err)

	}

	if d.options.StatusLogInterval > 0 {
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case <-time.After(d.options.StatusLogInterval):
				}

				lastHeartbeatReceived := time.Unix(0, d.wc.lastHeartbeatRecievedAt.Load()).UTC().Format(time.RFC3339)
				lastHeartbeatSend := time.Unix(0, d.wc.lastHeartbeatSentAt.Load()).UTC().Format(time.RFC3339)
				lastSequenceNumber := d.lastSequenceNumber.Load()
				sessionID := ""
				if d.sessionID != nil {
					sessionID = *d.sessionID
				}

				logger.Info(fmt.Sprintf(
					"Discord client status: heartbeatRecievedAt: %s, heartbeatSentAt: %s, sequenceNumber: %d, sessionId: %s",
					lastHeartbeatReceived, lastHeartbeatSend, lastSequenceNumber, sessionID),
					slog.String("heartbeatRecievedAt", lastHeartbeatReceived),
					slog.String("heartbeatSentAt", lastHeartbeatReceived),
					slog.Int64("sequenceNumber", lastSequenceNumber),
					slog.String("sessionId", sessionID),
				)
			}
		}()
	}

	return nil
}

func (d *Client) Error() <-chan error {
	return d.errorCh
}

func (d *Client) Guilds() []Guild {
	if !d.isRunning {
		return make([]Guild, 0)
	}

	<-d.loadingCh

	d.mu.RLock()
	defer d.mu.RUnlock()

	guilds := make([]Guild, 0, len(d.guilds))

	for _, g := range d.guilds {
		guilds = append(guilds, g)
	}

	return guilds
}

func (d *Client) Guild(guildID string) *Guild {
	if !d.isRunning {
		return nil
	}

	<-d.loadingCh

	d.mu.RLock()
	defer d.mu.RUnlock()

	guild, ok := d.guilds[guildID]
	if ok {
		return &guild
	}
	return nil
}

func (d *Client) User() *User {
	if !d.isRunning {
		return nil
	}

	<-d.loadingCh

	user := d.user.ToUser()
	return &user
}
