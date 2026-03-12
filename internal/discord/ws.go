package discord

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"runtime"
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
		return gatewayPayload{}, fmt.Errorf("Unable to marshal message: %v", err)
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

func (d *Client) initWSConnection() error {
	conn, _, err := websocket.DefaultDialer.DialContext(d.ctx, d.options.GatewayURL, nil)
	if err != nil {
		return fmt.Errorf("Unable to create WS connection to discord gateway: %v", err)
	}
	d.connection = conn

	go func() {
		err := d.startEventLoop()
		if err != nil {
			d.wsErrorCh <- err
		}
	}()

	identifyMessage, err := createPayload(2, identifyMessage{
		Token: "Bot " + d.token,
		Properties: identifyProperties{
			OS:      runtime.GOOS,
			Browser: d.options.ClientIdentifier,
		},
		Intents: d.intents,
	})
	if err != nil {
		return fmt.Errorf("Unable to create identify payload: %w", err)
	}

	err = d.connection.WriteJSON(identifyMessage)
	if err != nil {
		return fmt.Errorf("Unable to send identify message to discord gateway: %w", err)
	}

	go func() {
		select {
		case <-time.After(d.options.WSReadyTimeout):
			d.wsErrorCh <- fmt.Errorf("Client did not become ready after waiting for %.2f seconds", d.options.WSReadyTimeout.Seconds())
		case <-d.readyCh:
		case <-d.ctx.Done():
		}
	}()

	return nil
}

func (d *Client) closeWSConnection() error {
	err := d.connection.Close()
	if err != nil {
		return fmt.Errorf("unable to close connection to discord gateway: %w", err)
	}

	return nil
}

func (d *Client) startEventLoop() error {
	for {
		var payload gatewayPayload

		err := d.connection.ReadJSON(&payload)
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				return nil
			}
			return fmt.Errorf("Unable to read next message from discord gateway: %v", err)
		}

		switch payload.Opcode {
		case 0:
			err = d.handleEvent(payload)
			if err != nil {
				// Do not stop event loop when individual events throw errors
				logger.Error(fmt.Sprintf("Error while handling event: %v", err), slog.Any("error", err))
			}
		case 7:
			//TODO
			logger.Warn("Reconnection is not implemented")
		case 9:
			//TODO
			logger.Warn("Invalid session is not implemented")
		case 10:
			if d.isGreeted {
				logger.Warn("Gateway has already greeted client, ignoring hello message")
				break
			}

			var helloMessage helloMessage
			err := json.Unmarshal(payload.Data, &helloMessage)
			if err != nil {
				return fmt.Errorf("Error while parsing hello message: %v", err)
			}

			d.isGreeted = true
			go d.startHeartbeatLoop(helloMessage.HeartbeatInterval)
		case 11:
			now := time.Now()
			d.lastHeartbeatRecievedAt.Store(now.UnixNano())

			if d.lastHeartbeatSentAt.Load() == 0 {
				logger.Warn("Heartbeat response recieved before any heartbeats were sent")
				break
			}

			heartbeatDelay := time.Since(time.Unix(0, d.lastHeartbeatSentAt.Load()))
			if d.options.HeartbeatMaxDelay < heartbeatDelay {
				logger.Warn(fmt.Sprintf("Heartbeat response after long delay of %.2f seconds", d.options.HeartbeatMaxDelay.Seconds()))
			}

		default:
			logger.Error(fmt.Sprintf("Recieved an unrecognized Opcode from the gateway: %d", payload.Opcode), slog.Int("opcode", payload.Opcode))
		}
	}
}

func (d *Client) startHeartbeatLoop(intervalMilliseconds int) error {
	interval := time.Millisecond * time.Duration(intervalMilliseconds)
	initialJitter := time.Duration(float64(interval) * rand.Float64())

	select {
	case <-d.ctx.Done():
		return d.ctx.Err()
	case <-time.After(initialJitter):
	}

	ticker := time.NewTicker(interval)
	for {
		sentNano := d.lastHeartbeatSentAt.Load()
		recvNano := d.lastHeartbeatRecievedAt.Load()
		if sentNano != 0 && (recvNano == 0 || sentNano > recvNano) {
			logger.Warn("No heartbeat recieved after previous heartbeat was sent")
		}

		var data any = d.lastSequenceNumber.Load()
		if data == 0 {
			data = nil
		}

		payload, err := createPayload(1, data)
		if err != nil {
			return fmt.Errorf("Unable to create heartbeat payload: %v", err)
		}
		err = d.connection.WriteJSON(payload)
		if err != nil {
			return fmt.Errorf("Unable to send heartbeat to discord gateway: %v", err)
		}

		now := time.Now()
		d.lastHeartbeatSentAt.Store(now.UnixNano())

		select {
		case <-d.ctx.Done():
			return d.ctx.Err()
		case <-ticker.C:
		}
	}
}

func (d *Client) handleEvent(payload gatewayPayload) error {
	if payload.EventName == nil {
		return fmt.Errorf("Expected event to contain a name")
	}

	if payload.SequenceNumber == nil {
		return fmt.Errorf("Expected event to contain a sequence number")
	}

	d.lastSequenceNumber.Store(*payload.SequenceNumber)

	switch *payload.EventName {
	case "READY":
		var readyMessage readyMessage
		err := json.Unmarshal(payload.Data, &readyMessage)
		if err != nil {
			return fmt.Errorf("Unable to unmarshal ready message: %v", err)
		}
		d.user = &readyMessage.User
		d.resumeURL = &readyMessage.ResumeURL
		d.sessionID = &readyMessage.SessionID

		d.mu.Lock()
		for _, guild := range readyMessage.Guilds {
			d.unavailableGuilds[guild.ID] = struct{}{}
		}
		d.mu.Unlock()

		close(d.readyCh)
		if len(d.unavailableGuilds) == 0 {
			close(d.guildsLoadingCh)
		}

	case "GUILD_CREATE":
		var guild guildObject
		err := json.Unmarshal(payload.Data, &guild)
		if err != nil {
			return fmt.Errorf("Unable to unmarshal guild create message: %v", err)
		}

		d.mu.Lock()
		d.guilds[guild.ID] = guild.ToGuild()
		delete(d.unavailableGuilds, guild.ID)
		d.mu.Unlock()

		if len(d.unavailableGuilds) == 0 {
			close(d.guildsLoadingCh)
		}

	case "GUILD_UPDATE":
		var guild guildObject
		err := json.Unmarshal(payload.Data, &guild)
		if err != nil {
			return fmt.Errorf("Unable to unmarshal guild update message: %v", err)
		}

		d.mu.Lock()
		d.guilds[guild.ID] = guild.ToGuild()
		d.mu.Unlock()

	case "GUILD_DELETE":
		var g struct {
			ID string `json:"id"`
		}
		json.Unmarshal(payload.Data, &g)
		d.mu.Lock()
		delete(d.guilds, g.ID)
		d.mu.Unlock()

	case "VOICE_STATE_UPDATE":
		var voiceState voiceStateObject
		err := json.Unmarshal(payload.Data, &voiceState)
		if err != nil {
			return fmt.Errorf("Unable to unmarshal voice state update message: %v", err)
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
			return fmt.Errorf("Unable to unmarshal voice server update message: %v", err)
		}

		d.voiceMu.Lock()
		if session, ok := d.voiceJoinSessions[voiceServer.GuildID]; ok {
			session.voiceServerCh <- voiceServer
		}
		d.voiceMu.Unlock()

	default:
		logger.Warn(fmt.Sprintf("Recieved unimplemented event: %s", *payload.EventName))
	}

	return nil
}
