package discord

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

type gatewayPayload struct {
	Opcode         int             `json:"op"`
	Data           json.RawMessage `json:"d"`
	SequenceNumber *int64          `json:"s"`
	EventName      *string         `json:"t"`
}

func createPayload(op int, message any) (gatewayPayload, error) {
	rawMessage, err := json.Marshal(message)
	if err != nil {
		return gatewayPayload{}, fmt.Errorf("unable to marshal message: %v", err)
	}

	return gatewayPayload{
		Opcode: op,
		Data:   rawMessage,
	}, nil
}

type helloMessage struct {
	HeartbeatInterval int `json:"heartbeat_interval"`
}

type readyMessage struct {
	Version   int                      `json:"v"`
	User      userObject               `json:"user"`
	Guilds    []unavailableGuildObject `json:"guilds"`
	SessionID string                   `json:"session_id"`
	ResumeURL string                   `json:"resume_gateway_url"`
}

type voiceStateUpdateMessage struct {
	GuildID   string  `json:"guild_id"`
	ChannelID *string `json:"channel_id"`
	SelfMute  bool    `json:"self_mute"`
	SelfDeaf  bool    `json:"self_deaf"`
}

type identifyMessage struct {
	Token      string             `json:"token"`
	Properties identifyProperties `json:"properties"`
	Intents    int                `json:"intents"`
}

type identifyProperties struct {
	OS      string `json:"$os"`
	Browser string `json:"$browser"`
	Device  string `json:"$device"`
}

type resumeMessage struct {
	Token     string `json:"token"`
	SessionID string `json:"session_id"`
	Sequence  int    `json:"seq"`
}

type websocketClient struct {
	client *Client

	ctx                     context.Context
	connection              *websocket.Conn
	lastHeartbeatSentAt     atomic.Int64
	lastHeartbeatRecievedAt atomic.Int64
	isGreeted               bool

	readyCh chan struct{}
	errorCh chan error
}

func (d *Client) startWebsockets() error {
	err := d.startNewWebsocketClient(nil)
	if err != nil {
		return err
	}

	go func() {
		for {
			select {
			case <-d.ctx.Done():
				return
			case err = <-d.wc.errorCh:
			}

			err := d.startNewWebsocketClient(err)
			if err != nil {
				d.errorCh <- err
				return
			}
		}
	}()

	return nil
}

func (d *Client) startNewWebsocketClient(previousError error) error {
	if previousError == nil {
	} else if closeError, ok := previousError.(*websocket.CloseError); ok {
		switch closeError.Code {
		case 1006, 4000, 4001, 4002, 4005, 4008:
			logger.Warn(fmt.Sprintf("Websocket closed with code %d, reconnecting", closeError.Code), slog.Int("ws_code", closeError.Code))
		case 4003, 4007, 4009:
			logger.Warn(fmt.Sprintf("Websocket closed with code %d, reconnecting with new session", closeError.Code), slog.Int("ws_code", closeError.Code))
			d.sessionID = nil
		case 4004, 4010, 4011, 4012, 4013, 4014:
			return fmt.Errorf("websocket closed with code %d, closing client", closeError.Code)
		default:
			return fmt.Errorf("websocket closed with unrecognized code %d, closing client", closeError.Code)
		}
	} else {
		return fmt.Errorf("error while running websocket client: %v", previousError)
	}

	wc := d.newWSClient()
	d.wc = &wc
	err := d.wc.start()
	if err != nil {
		return fmt.Errorf("error while starting websocket client: %v", err)
	}

	return nil
}

func (d *Client) sendWSMessage(payload gatewayPayload) error {
	<-d.wc.readyCh

	// During reconnects we may get socket closed errors even if we wait for the ready channel
	// This handles those gracefully
	for range d.options.WebsocketRetryLimit {
		err := d.wc.connection.WriteJSON(payload)
		if errors.Is(err, net.ErrClosed) {
			<-time.After(d.options.WSReadyTimeout)
			<-d.wc.readyCh
		} else {
			return err
		}
	}

	return d.wc.connection.WriteJSON(payload)
}

func (d *Client) newWSClient() websocketClient {
	return websocketClient{
		client:  d,
		readyCh: make(chan struct{}),
		errorCh: make(chan error),
	}
}

func (wc *websocketClient) start() error {
	ctx, cancel := context.WithCancel(wc.client.ctx)

	url := *wc.client.gatewayURL
	if wc.client.resumeURL != nil {
		url = *wc.client.resumeURL
	}

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, url+"?v=10&encoding=json", nil)
	if err != nil {
		cancel()
		return fmt.Errorf("unable to create WS connection to discord gateway: %w", err)
	}

	wc.ctx = ctx
	wc.connection = conn

	go func() {
		<-ctx.Done()
		wc.close()
	}()

	go func() {
		err := wc.runEventLoop()
		cancel()
		wc.errorCh <- err
	}()

	err = wc.createOrResumeSession()
	if err != nil {
		return err
	}

	select {
	case <-time.After(wc.client.options.WSReadyTimeout):
		return fmt.Errorf("websocket client did not become ready after waiting for %.2f seconds", wc.client.options.WSReadyTimeout.Seconds())
	case <-wc.readyCh:
		return nil
	case err = <-wc.errorCh:
		return err
	case <-wc.ctx.Done():
		return wc.ctx.Err()
	}
}

func (wc *websocketClient) createOrResumeSession() error {
	if wc.client.sessionID == nil {
		identifyMessage, err := createPayload(2, identifyMessage{
			Token: "Bot " + wc.client.token,
			Properties: identifyProperties{
				OS:      runtime.GOOS,
				Browser: wc.client.options.ClientIdentifier,
			},
			Intents: wc.client.intents,
		})
		if err != nil {
			return fmt.Errorf("unable to create identify payload: %w", err)
		}

		err = wc.connection.WriteJSON(identifyMessage)
		if err != nil {
			return fmt.Errorf("unable to send identify message to discord gateway: %w", err)
		}
	} else {
		resumeMessage, err := createPayload(6, resumeMessage{
			Token:     "Bot " + wc.client.token,
			SessionID: *wc.client.sessionID,
			Sequence:  int(wc.client.lastSequenceNumber.Load()),
		})
		if err != nil {
			return fmt.Errorf("unable to create resume payload: %w", err)
		}

		err = wc.connection.WriteJSON(resumeMessage)
		if err != nil {
			return fmt.Errorf("unable to send resume message to discord gateway: %w", err)
		}
	}

	return nil
}

func (wc *websocketClient) runEventLoop() error {
	for {
		var payload gatewayPayload

		err := wc.connection.ReadJSON(&payload)
		if err != nil {
			return err
		}

		if !wc.isGreeted && payload.Opcode != 10 {
			return fmt.Errorf("expected first message to be a hello message")
		}

		switch payload.Opcode {
		case 0:
			err = wc.client.handleEvent(payload)
			if err != nil {
				// Do not stop event loop when individual events throw errors
				logger.Error(fmt.Sprintf("Error while handling event: %v", err), slog.Any("error", err))
			}
		case 7:
			logger.Warn("Recieved reconnect message from gateway, reconnecting to websocket server...")
			err := wc.connection.Close()
			return err
		case 9:
			var resumable bool
			err := json.Unmarshal(payload.Data, &resumable)
			if err != nil {
				return fmt.Errorf("error while parsing invalid session message: %v", err)
			}

			if !resumable {
				logger.Warn("Recieved invalid session message from gateway, reconnecting to websocket server with new session...")
				wc.client.sessionID = nil
			} else {
				logger.Warn("Recieved invalid session event from gateway, reconnecting to websocket server...")
			}

			err = wc.connection.Close()
			return err
		case 10:
			if wc.isGreeted {
				logger.Warn("Gateway has already greeted client, ignoring hello message")
				break
			}

			logger.Info("Recieved hello message from gateway")

			var helloMessage helloMessage
			err := json.Unmarshal(payload.Data, &helloMessage)
			if err != nil {
				return fmt.Errorf("error while parsing hello message: %v", err)
			}

			wc.isGreeted = true
			go wc.runHeartbeatLoop(helloMessage.HeartbeatInterval)
		case 11:
			now := time.Now()
			wc.lastHeartbeatRecievedAt.Store(now.UnixNano())

			if wc.lastHeartbeatSentAt.Load() == 0 {
				logger.Warn("Heartbeat response recieved before any heartbeats were sent")
				break
			}

			heartbeatDelay := time.Since(time.Unix(0, wc.lastHeartbeatSentAt.Load()))
			if wc.client.options.HeartbeatMaxDelay < heartbeatDelay {
				logger.Warn(fmt.Sprintf("Heartbeat response after long delay of %.2f seconds", wc.client.options.HeartbeatMaxDelay.Seconds()))
			}

		default:
			logger.Error(fmt.Sprintf("Recieved an unrecognized Opcode from the gateway: %d", payload.Opcode), slog.Int("opcode", payload.Opcode))
		}
	}
}

func (wc *websocketClient) runHeartbeatLoop(intervalMilliseconds int) {
	interval := time.Millisecond * time.Duration(intervalMilliseconds)
	initialJitter := time.Duration(float64(interval) * rand.Float64())

	select {
	case <-wc.ctx.Done():
		return
	case <-time.After(initialJitter):
	}

	ticker := time.NewTicker(interval)
	for {
		sentNano := wc.lastHeartbeatSentAt.Load()
		recvNano := wc.lastHeartbeatRecievedAt.Load()
		if sentNano != 0 && (recvNano == 0 || sentNano > recvNano) {
			logger.Warn("No heartbeat recieved after previous heartbeat was sent")
		}

		var data any = wc.client.lastSequenceNumber.Load()
		if data == 0 {
			data = nil
		}

		payload, err := createPayload(1, data)
		if err != nil {
			logger.Error(fmt.Sprintf("unable to create heartbeat payload: %v", err), slog.Any("error", err))
			wc.close()
			return
		}

		err = wc.connection.WriteJSON(payload)
		if err != nil {
			logger.Error(fmt.Sprintf("unable to send heartbeat to discord gateway: %v", err), slog.Any("error", err))
			wc.close()
			return
		}

		now := time.Now()
		wc.lastHeartbeatSentAt.Store(now.UnixNano())

		select {
		case <-wc.ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (d *Client) handleEvent(payload gatewayPayload) error {
	if payload.EventName == nil {
		return fmt.Errorf("expected event to contain a name")
	}

	if payload.SequenceNumber == nil {
		return fmt.Errorf("expected event to contain a sequence number")
	}

	logger.Info(fmt.Sprintf("Recieved event message from gateway, event name: %s", *payload.EventName), slog.String("event_name", *payload.EventName))

	d.lastSequenceNumber.Store(*payload.SequenceNumber)

	switch *payload.EventName {
	case "READY":
		var readyMessage readyMessage
		err := json.Unmarshal(payload.Data, &readyMessage)
		if err != nil {
			return fmt.Errorf("unable to unmarshal ready message: %v", err)
		}
		d.user = &readyMessage.User
		d.resumeURL = &readyMessage.ResumeURL
		d.sessionID = &readyMessage.SessionID

		d.mu.Lock()
		for _, guild := range readyMessage.Guilds {
			d.unavailableGuilds[guild.ID] = struct{}{}
		}
		d.mu.Unlock()

		close(d.wc.readyCh)
		if len(d.unavailableGuilds) == 0 {
			close(d.loadingCh)
		}

	case "RESUMED":
		close(d.wc.readyCh)

	case "GUILD_CREATE":
		var guild guildObject
		err := json.Unmarshal(payload.Data, &guild)
		if err != nil {
			return fmt.Errorf("unable to unmarshal guild create message: %w", err)
		}

		d.mu.Lock()
		d.guilds[guild.ID] = guild.ToGuild()
		delete(d.unavailableGuilds, guild.ID)
		d.mu.Unlock()

		if len(d.unavailableGuilds) == 0 {
			close(d.loadingCh)
		}

	case "GUILD_UPDATE":
		var guild guildObject
		err := json.Unmarshal(payload.Data, &guild)
		if err != nil {
			return fmt.Errorf("unable to unmarshal guild update message: %w", err)
		}

		d.mu.Lock()
		d.guilds[guild.ID] = guild.ToGuild()
		d.mu.Unlock()

	case "GUILD_DELETE":
		var g struct {
			ID string `json:"id"`
		}
		err := json.Unmarshal(payload.Data, &g)
		if err != nil {
			return fmt.Errorf("unable to unmarshal guild delete message: %w", err)
		}

		d.mu.Lock()
		delete(d.guilds, g.ID)
		d.mu.Unlock()

	case "VOICE_STATE_UPDATE":
		var voiceState voiceStateObject
		err := json.Unmarshal(payload.Data, &voiceState)
		if err != nil {
			return fmt.Errorf("unable to unmarshal voice state update message: %w", err)
		}

		d.voiceMu.Lock()
		if session, ok := d.voiceJoinSessions[voiceState.GuildID]; ok {
			session.sessionIdCh <- voiceState.SessionID
		}
		d.voiceMu.Unlock()

		d.mu.Lock()
		guild, ok := d.guilds[voiceState.GuildID]
		if ok {
			guild.VoiceStates[voiceState.UserID] = VoiceState{
				ChannelID: voiceState.ChannelID,
				UserID:    voiceState.UserID,
				SessionID: voiceState.SessionID,
			}
		} else {
			logger.Warn(fmt.Sprintf("Got voice state for unknown guild: %s", voiceState.GuildID), slog.String("guild_id", voiceState.GuildID))
		}
		d.mu.Unlock()

	case "VOICE_SERVER_UPDATE":
		var voiceServer voiceServerObject
		err := json.Unmarshal(payload.Data, &voiceServer)
		if err != nil {
			return fmt.Errorf("unable to unmarshal voice server update message: %w", err)
		}

		d.voiceMu.Lock()
		if session, ok := d.voiceJoinSessions[voiceServer.GuildID]; ok {
			session.voiceServerCh <- voiceServer
		}
		d.voiceMu.Unlock()
	}

	return nil
}

func (wc *websocketClient) close() {
	err := wc.connection.Close()
	if err != nil && !errors.Is(err, net.ErrClosed) {
		logger.Error(fmt.Sprintf("unable to close connection to discord gateway: %v", err), slog.Any("error", err))
	}
}
