package discord

type guildObject struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Unavailable bool               `json:"unavailable"`
	MemberCount int                `json:"member_count"`
	Channels    []channelObject    `json:"channels"`
	VoiceStates []voiceStateObject `json:"voice_states"`
}

func (g guildObject) ToGuild() Guild {
	voiceStates := make(map[string]VoiceState)
	for _, vs := range g.VoiceStates {
		voiceStates[vs.UserID] = vs.ToVoiceState()
	}

	channels := make([]Channel, len(g.Channels))
	for i, c := range g.Channels {
		channels[i] = c.ToChannel()
	}

	return Guild{
		ID:          g.ID,
		Name:        g.Name,
		VoiceStates: voiceStates,
		Channels:    channels,
	}
}

type channelObject struct {
	ID                   string                      `json:"id"`
	Type                 int                         `json:"type"`
	GuildId              string                      `json:"guild_id"`
	Position             int                         `json:"position"`
	PermissionOverwrites []permissionOverwriteObject `json:"permission_overwrites"`
	Name                 *string                     `json:"name"`
}

func (c channelObject) ToChannel() Channel {
	permissionOverwrites := make([]PermissionOverwrite, len(c.PermissionOverwrites))
	for i, po := range c.PermissionOverwrites {
		permissionOverwrites[i] = po.ToPermissionOverwrite()
	}

	return Channel{
		ID:                   c.ID,
		Type:                 ChannelType(c.Type),
		GuildID:              c.GuildId,
		Position:             c.Position,
		PermissionOverwrites: permissionOverwrites,
		Name:                 c.Name,
	}
}

type permissionOverwriteObject struct {
	ID    string                  `json:"id"`
	Type  PermissionOverwriteType `json:"type"`
	Allow Permission              `json:"allow"`
	Deny  Permission              `json:"deny"`
}

func (po permissionOverwriteObject) ToPermissionOverwrite() PermissionOverwrite {
	return PermissionOverwrite(po)
}

type unavailableGuildObject struct {
	ID          string `json:"id"`
	Unavailable bool   `json:"unavailable"`
}

type userObject struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	Discriminator string `json:"discriminator"`
	GlobalName    string `json:"global_name"`
	Avatar        string `json:"avatar"`
	Bot           bool   `json:"bot"`
	System        bool   `json:"system"`
	MFAEnabled    bool   `json:"mfa_enabled"`
	Banner        string `json:"banner"`
	AccentColor   int    `json:"accent_color"`
	Locale        string `json:"locale"`
	Verified      bool   `json:"verified"`
	Email         string `json:"email"`
	Flags         int    `json:"flags"`
	PremiumType   int    `json:"premium_type"`
	PublicFlags   int    `json:"public_flags"`
}

func (u *userObject) ToUser() User {
	return User{
		ID:            u.ID,
		Username:      u.Username,
		Discriminator: u.Discriminator,
		GlobalName:    u.GlobalName,
		Avatar:        u.Avatar,
		Bot:           u.Bot,
	}
}

type voiceStateObject struct {
	GuildID    string  `json:"guild_id"`
	ChannelID  *string `json:"channel_id"`
	UserID     string  `json:"user_id"`
	SessionID  string  `json:"session_id"`
	Deaf       bool    `json:"deaf"`
	Mute       bool    `json:"mute"`
	SelfDeaf   bool    `json:"self_deaf"`
	SelfMute   bool    `json:"self_mute"`
	SelfStream bool    `json:"self_stream"`
	SelfVideo  bool    `json:"self_video"`
	Suppress   bool    `json:"suppress"`
}

func (vs *voiceStateObject) ToVoiceState() VoiceState {
	return VoiceState{
		ChannelID: vs.ChannelID,
		UserID:    vs.UserID,
		SessionID: vs.SessionID,
	}
}

type voiceServerObject struct {
	Token    string `json:"token"`
	GuildID  string `json:"guild_id"`
	Endpoint string `json:"endpoint"`
}

type soundboardSoundObject struct {
	Name      string  `json:"name"`
	SoundID   string  `json:"sound_id"`
	Volume    float64 `json:"volume"`
	EmojiID   string  `json:"emoji_id"`
	EmojiName string  `json:"emoji_name"`
	GuildID   *string `json:"guild_id"`
	Available bool    `json:"available"`
}

func (ss *soundboardSoundObject) ToSoundboardSound() SoundboardSound {
	return SoundboardSound{
		Name:      ss.Name,
		SoundID:   ss.SoundID,
		Volume:    ss.Volume,
		EmojiID:   ss.EmojiID,
		EmojiName: ss.EmojiName,
		GuildID:   ss.GuildID,
		Available: ss.Available,
	}
}
