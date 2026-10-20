package discord

import (
	"fmt"
	"time"
)

func (d *Client) JoinVoiceChannel(guildID string, channelID string, selfMute bool, selfDeaf bool) (*DiscordVoiceClient, error) {
	return d.joinVoiceChannel(guildID, channelID, selfMute, selfDeaf, true)
}

// joinVoiceChannel joins a voice channel. A fresh join waits for both the
// bot's voice state and the voice server; switching channels only waits for
// the voice state, as Discord needn't send a new voice server.
func (d *Client) joinVoiceChannel(guildID string, channelID string, selfMute bool, selfDeaf bool, needServer bool) (*DiscordVoiceClient, error) {
	js, err := d.createJoinSession(guildID)
	if err != nil {
		return nil, err
	}

	defer func() {
		d.voiceMu.Lock()
		delete(d.voiceJoinSessions, guildID)
		d.voiceMu.Unlock()
	}()

	voiceStateUpdateMessage, err := createPayload(4, voiceStateUpdateMessage{
		GuildID:   guildID,
		ChannelID: &channelID,
		SelfMute:  selfMute,
		SelfDeaf:  selfDeaf,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to create voice state update payload: %v", err)
	}

	err = d.sendWSMessage(voiceStateUpdateMessage)
	if err != nil {
		return nil, fmt.Errorf("unable to send voice state update message to discord gateway: %v", err)
	}

	var sessionID string
	var voiceServer voiceServerObject
	gotServer := !needServer
	timeout := time.After(d.options.VoiceJoinTimeout)
	for sessionID == "" || !gotServer {
		select {
		case sID := <-js.sessionIdCh:
			sessionID = sID
		case vs := <-js.voiceServerCh:
			voiceServer = vs
			gotServer = true
		case <-timeout:
			return nil, fmt.Errorf("voice connection timed out waiting %.2f seconds gateway events", d.options.VoiceJoinTimeout.Seconds())
		case <-d.ctx.Done():
			return nil, d.ctx.Err()
		}
	}

	vc := DiscordVoiceClient{
		d:           d,
		guildID:     guildID,
		sessionID:   sessionID,
		voiceServer: voiceServer,
	}

	return &vc, nil
}

func (vc *DiscordVoiceClient) ChangeChannel(channelID string, selfMute bool, selfDeaf bool) error {
	newVC, err := vc.d.joinVoiceChannel(vc.guildID, channelID, selfMute, selfDeaf, false)
	if err != nil {
		return fmt.Errorf("unable to join voice channel: %v", err)
	}

	vc.sessionID = newVC.sessionID
	if newVC.voiceServer.GuildID != "" {
		vc.voiceServer = newVC.voiceServer
	}

	return nil
}

type voiceUpdateSession struct {
	sessionIdCh   chan string
	voiceServerCh chan voiceServerObject
}

func (d *Client) createJoinSession(guildID string) (*voiceUpdateSession, error) {
	d.voiceMu.Lock()
	defer d.voiceMu.Unlock()

	if _, ok := d.voiceJoinSessions[guildID]; ok {
		return nil, fmt.Errorf("unable to connection to voice because join session already exists for guild: %s", guildID)
	}

	joinSession := voiceUpdateSession{
		sessionIdCh:   make(chan string, 1),
		voiceServerCh: make(chan voiceServerObject, 1),
	}
	d.voiceJoinSessions[guildID] = joinSession

	return &joinSession, nil
}

type DiscordVoiceClient struct {
	guildID     string
	voiceServer voiceServerObject
	sessionID   string

	d *Client
}

func (vc *DiscordVoiceClient) Leave() error {
	payload, err := createPayload(4, voiceStateUpdateMessage{
		GuildID:   vc.guildID,
		ChannelID: nil,
		SelfMute:  false,
		SelfDeaf:  false,
	})
	if err != nil {
		return fmt.Errorf("unable to create voice leave payload: %v", err)
	}

	err = vc.d.sendWSMessage(payload)
	if err != nil {
		return fmt.Errorf("unable to send voice leave message to discord gateway: %v", err)
	}

	return nil
}
