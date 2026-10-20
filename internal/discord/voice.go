package discord

import (
	"fmt"
	"time"
)

// JoinVoiceChannel joins (or switches to) a voice channel. It returns once the
// bot's own voice state shows it in that channel; the janitor never sends
// audio, so it doesn't wait for a voice server, which Discord needn't send
// when switching channels.
func (d *Client) JoinVoiceChannel(guildID string, channelID string, selfMute bool, selfDeaf bool) (*DiscordVoiceClient, error) {
	js, err := d.createJoinSession(guildID, channelID)
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
	timeout := time.After(d.options.VoiceJoinTimeout)
	for sessionID == "" {
		select {
		case sID := <-js.sessionIdCh:
			sessionID = sID
		case vs := <-js.voiceServerCh:
			voiceServer = vs
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
	newVC, err := vc.d.JoinVoiceChannel(vc.guildID, channelID, selfMute, selfDeaf)
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
	channelID     string // the channel being joined
	sessionIdCh   chan string
	voiceServerCh chan voiceServerObject
}

func (d *Client) createJoinSession(guildID, channelID string) (*voiceUpdateSession, error) {
	d.voiceMu.Lock()
	defer d.voiceMu.Unlock()

	if _, ok := d.voiceJoinSessions[guildID]; ok {
		return nil, fmt.Errorf("unable to connection to voice because join session already exists for guild: %s", guildID)
	}

	joinSession := voiceUpdateSession{
		channelID:     channelID,
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
