package discord

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/robert-mccausland/janitor-bot/internal/logging"
)

var (
	logger = logging.NewLogger("github.com/robert-mccausland/janitor-bot/internal/discord")
)

type Client struct {
	token   string
	intents int
	options ClientOptions

	ctx context.Context

	httpClient              *http.Client
	connection              *websocket.Conn
	user                    *userObject
	sessionID               *string
	resumeURL               *string
	lastSequenceNumber      atomic.Int64
	lastHeartbeatSentAt     atomic.Int64
	lastHeartbeatRecievedAt atomic.Int64
	isGreeted               bool
	isRunning               bool

	wsErrorCh       chan error
	startingCh      chan struct{}
	readyCh         chan struct{}
	guildsLoadingCh chan struct{}

	mu                sync.RWMutex
	guilds            map[string]Guild
	unavailableGuilds map[string]struct{}

	voiceMu           sync.RWMutex
	voiceJoinSessions map[string]voiceUpdateSession
}

type ClientOptions struct {
	ApiURL                    string
	ClientIdentifier          string
	GatewayURL                string
	HeartbeatMaxDelay         time.Duration
	WSReadyTimeout            time.Duration
	VoiceJoinTimeout          time.Duration
	RestAPITimeout            time.Duration
	HttpIdleConnectionTimeout time.Duration
	HttpMaxIdleConnections    int
}

func DefaultOptions() ClientOptions {
	return ClientOptions{
		ApiURL:                    "https://discord.com/api/v10",
		GatewayURL:                "wss://gateway.discord.gg/?v=10&encoding=json",
		ClientIdentifier:          "janitor-bot-client",
		HeartbeatMaxDelay:         5 * time.Second,
		WSReadyTimeout:            10 * time.Second,
		VoiceJoinTimeout:          10 * time.Second,
		RestAPITimeout:            10 * time.Second,
		HttpIdleConnectionTimeout: 60 * time.Second,
		HttpMaxIdleConnections:    10,
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
		guildsLoadingCh:   make(chan struct{}),
		startingCh:        make(chan struct{}),
		readyCh:           make(chan struct{}),
		wsErrorCh:         make(chan error),
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

func (d *Client) Run(ctx context.Context, token string, intents int) error {
	if d.isRunning {
		return fmt.Errorf("discord client is already running")
	}

	ctx, cancel := context.WithCancel(ctx)

	d.isRunning = true
	d.token = token
	d.intents = intents
	d.ctx = ctx

	defer cancel()

	close(d.startingCh)

	err := d.initWSConnection()
	if err != nil {
		return fmt.Errorf("unable to initialize websocket connection: %w", err)
	}
	defer func() {
		err := d.closeWSConnection()
		if err != nil {
			logger.Error(fmt.Sprintf("Error while closing websocket connection: %v", err), slog.Any("error", err))
		}
	}()

	return d.WaitForShutdown()
}

func (d *Client) WaitForReady() error {
	<-d.startingCh

	select {
	case <-d.readyCh:
		return nil
	case err := <-d.wsErrorCh:
		return fmt.Errorf("web socket error: %w", err)
	case <-d.ctx.Done():
		return d.ctx.Err()
	}
}

func (d *Client) WaitForShutdown() error {
	<-d.startingCh

	select {
	case err := <-d.wsErrorCh:
		return fmt.Errorf("web socket error: %w", err)
	case <-d.ctx.Done():
		return d.ctx.Err()
	}
}

func (d *Client) Guilds() []Guild {
	if !d.isRunning {
		return make([]Guild, 0)
	}

	<-d.guildsLoadingCh

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

	<-d.guildsLoadingCh

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

	<-d.readyCh

	user := d.user.ToUser()
	return &user
}
