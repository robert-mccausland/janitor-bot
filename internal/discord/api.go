package discord

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

func (d *Client) GetChannels(guildID string) (*[]Channel, error) {
	if d.Guild(guildID) == nil {
		return nil, fmt.Errorf("could not find guild with ID: %s", guildID)
	}

	var response []channelObject
	err := d.doRequest("GET", fmt.Sprintf("/guilds/%s/channels", guildID), nil, &response)
	if err != nil {
		return nil, fmt.Errorf("unable to get guild channels from REST API: %w", err)
	}

	channels := make([]Channel, len(response))
	for i, c := range response {
		channels[i] = c.ToChannel()
	}

	return &channels, nil
}

func (d *Client) GetChannel(channelID string) (*Channel, error) {

	var response channelObject
	err := d.doRequest("GET", fmt.Sprintf("/channels/%s", channelID), nil, &response)
	if err != nil {
		return nil, fmt.Errorf("unable to get channel from REST API: %w", err)
	}

	channel := response.ToChannel()
	return &channel, nil
}

func (d *Client) EditChannelPermissions(channelID string, overwrite PermissionOverwrite) error {
	body := struct {
		Allow Permission              `json:"allow"`
		Deny  Permission              `json:"deny"`
		Type  PermissionOverwriteType `json:"type"`
	}{
		Allow: overwrite.Allow,
		Deny:  overwrite.Deny,
		Type:  overwrite.Type,
	}

	err := d.doRequest("PUT", fmt.Sprintf("/channels/%s/permissions/%s", channelID, overwrite.ID), body, nil)
	if err != nil {
		return fmt.Errorf("unable to edit channel permissions: %w", err)
	}

	return nil
}

func (d *Client) DeleteChannelPermissions(channelID string, overwriteID string) error {
	err := d.doRequest("DELETE", fmt.Sprintf("/channels/%s/permissions/%s", channelID, overwriteID), nil, nil)
	if err != nil {
		return fmt.Errorf("unable to delete channel permissions: %w", err)
	}

	return nil
}

type GuildMemberUpdate struct {
	Nick                       *string
	Roles                      *[]string
	Mute                       *bool
	Deaf                       *bool
	CommunicationDisabledUntil *time.Time
	Flags                      *int

	// Use an empty string to disconnect the user form voice
	ChannelID *string
}

func (d *Client) ModifyGuildMember(guildID string, userID string, update GuildMemberUpdate) error {
	body := struct {
		Nick                       *string    `json:"nick,omitempty"`
		Roles                      *[]string  `json:"roles,omitempty"`
		Mute                       *bool      `json:"mute,omitempty"`
		Deaf                       *bool      `json:"deaf,omitempty"`
		ChannelID                  *string    `json:"channel_id,omitempty"`
		CommunicationDisabledUntil *time.Time `json:"communication_disabled_until,omitempty"`
		Flags                      *int       `json:"flags,omitempty"`
	}{
		Nick:                       update.Nick,
		Roles:                      update.Roles,
		Mute:                       update.Mute,
		Deaf:                       update.Deaf,
		ChannelID:                  update.ChannelID,
		CommunicationDisabledUntil: update.CommunicationDisabledUntil,
		Flags:                      update.Flags,
	}

	err := d.doRequest("PATCH", fmt.Sprintf("/guilds/%s/members/%s", guildID, userID), body, nil)
	if err != nil {
		return fmt.Errorf("unable to modify guild member: %w", err)
	}

	return nil
}

type SoundboardSound struct {
	SoundID       string
	SourceGuildID *string
}

func (d *Client) SendSoundboardSound(channelID string, sound SoundboardSound) error {
	body := struct {
		SoundID       string  `json:"sound_id"`
		SourceGuildID *string `json:"source_guild_id,omitempty"`
	}{
		SoundID:       sound.SoundID,
		SourceGuildID: sound.SourceGuildID,
	}

	err := d.doRequest("POST", fmt.Sprintf("/channels/%s/send-soundboard-sound", channelID), body, nil)
	if err != nil {
		return fmt.Errorf("unable to send soundboard sound: %w", err)
	}

	return nil
}

func (d *Client) getGatewayURL() (string, error) {
	var response struct {
		URL string `json:"url"`
	}

	err := d.doRequest("GET", "/gateway", nil, &response)
	if err != nil {
		return "", fmt.Errorf("unable to get gateway URL from REST API: %w", err)
	}

	return response.URL, nil
}

type discordAPIError struct {
	Message string          `json:"message"`
	Code    int             `json:"code"`
	Errors  json.RawMessage `json:"errors"`
}

func (d *Client) doRequest(method, endpoint string, body any, responseValue any) error {
	url := d.options.ApiURL + endpoint

	requestReader, requestWriter := io.Pipe()
	if body != nil {
		go func() {
			err := json.NewEncoder(requestWriter).Encode(body)

			var closeErr error
			if err != nil {
				closeErr = requestWriter.CloseWithError(err)
			} else {
				closeErr = requestWriter.Close()
			}

			if closeErr != nil {
				logger.Error(fmt.Sprintf("Unable to close writer: %v", closeErr), slog.Any("error", closeErr))
			}
		}()
	} else {
		closeErr := requestWriter.Close()
		if closeErr != nil {
			logger.Error(fmt.Sprintf("Unable to close writer: %v", closeErr), slog.Any("error", closeErr))
		}
	}

	req, err := http.NewRequestWithContext(d.ctx, method, url, requestReader)
	if err != nil {
		closeErr := requestReader.Close()
		if closeErr != nil {
			logger.Error(fmt.Sprintf("Unable to close reader: %v", closeErr), slog.Any("error", closeErr))
		}
		return fmt.Errorf("unable to create API request: %w", err)
	}

	req.Header.Set("Authorization", "Bot "+d.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", d.options.ClientIdentifier)

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("unable to send API request: %w", err)
	}
	defer func() {
		closeErr := resp.Body.Close()
		if closeErr != nil {
			logger.Error(fmt.Sprintf("Unable to close response body: %v", closeErr), slog.Any("error", closeErr))
		}
	}()

	if resp.StatusCode >= 400 {
		var apiError discordAPIError
		if err := json.NewDecoder(resp.Body).Decode(&apiError); err != nil {
			return fmt.Errorf("recieved status code %d from discord api (could not decode error)", resp.StatusCode)
		}
		return fmt.Errorf("recieved error from discord api: %s (%d)", apiError.Message, apiError.Code)
	}

	if responseValue != nil {
		err = json.NewDecoder(resp.Body).Decode(responseValue)
		if err != nil {
			return fmt.Errorf("unable to parse API response: %w", err)
		}
	}

	return nil
}

func (p *Permission) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}

	val, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return fmt.Errorf("unable to parse permission bitmask: %w", err)
	}

	*p = Permission(val)
	return nil
}

func (p Permission) MarshalJSON() ([]byte, error) {
	return json.Marshal(strconv.FormatUint(uint64(p), 10))
}
