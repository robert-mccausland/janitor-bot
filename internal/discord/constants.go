package discord

// https://docs.discord.com/developers/events/gateway#gateway-intents
type Intent int

const (
	IntentGuilds                      Intent = 1 << 0
	IntentGuildMembers                Intent = 1 << 1
	IntentGuildModeration             Intent = 1 << 2
	IntentGuildExpressions            Intent = 1 << 3
	IntentGuildIntegrations           Intent = 1 << 4
	IntentGuildWebhooks               Intent = 1 << 5
	IntentGuildInvites                Intent = 1 << 6
	IntentGuildVoiceStates            Intent = 1 << 7
	IntentGuildPresences              Intent = 1 << 8
	IntentGuildMessages               Intent = 1 << 9
	IntentGuildMessageReactions       Intent = 1 << 10
	IntentGuildMessageTyping          Intent = 1 << 11
	IntentDirectMessages              Intent = 1 << 12
	IntentDirectMessageReactions      Intent = 1 << 13
	IntentDirectMessageTyping         Intent = 1 << 14
	IntentMessageContent              Intent = 1 << 15
	IntentGuildScheduledEvents        Intent = 1 << 16
	IntentAutoModerationConfiguration Intent = 1 << 20
	IntentAutoModerationExecution     Intent = 1 << 21
	IntentGuildMessagePolls           Intent = 1 << 24
	IntentDirectMessagePolls          Intent = 1 << 25

	IntentsStandard = IntentGuilds |
		IntentGuildModeration |
		IntentGuildExpressions |
		IntentGuildIntegrations |
		IntentGuildWebhooks |
		IntentGuildInvites |
		IntentGuildVoiceStates |
		IntentGuildMessages |
		IntentGuildMessageReactions |
		IntentGuildMessageTyping |
		IntentDirectMessages |
		IntentDirectMessageReactions |
		IntentDirectMessageTyping |
		IntentGuildScheduledEvents |
		IntentAutoModerationConfiguration |
		IntentAutoModerationExecution

	IntentsPrivileged = IntentGuildMembers |
		IntentGuildPresences |
		IntentMessageContent

	IntentsAll = IntentsStandard | IntentsPrivileged

	IntentsNone Intent = 0
)

// https://docs.discord.com/developers/resources/channel#channel-object-channel-types
type ChannelType int

const (
	ChannelTypeGuildText               ChannelType = 0
	ChannelTypeDM                      ChannelType = 1
	ChannelTypeGuildVoice              ChannelType = 2
	ChannelTypeGroupDM                 ChannelType = 3
	ChannelTypeGuildCategory           ChannelType = 4
	ChannelTypeGuildAnnouncement       ChannelType = 5
	ChannelTypeGuildAnnouncementThread ChannelType = 10
	ChannelTypeGuildPublicThread       ChannelType = 11
	ChannelTypeGuildPrivateThread      ChannelType = 12
	ChannelTypeGuildStageVoice         ChannelType = 13
	ChannelTypeGuildDirectory          ChannelType = 14
	ChannelTypeGuildForum              ChannelType = 15
	ChannelTypeGuildMedia              ChannelType = 16
)

type PermissionOverwriteType int

const (
	PermissionOverwriteTypeRole   PermissionOverwriteType = 0
	PermissionOverwriteTypeMember PermissionOverwriteType = 1
)

type Permission uint64

const (
	PermissionNone                             Permission = 0
	PermissionCreateInstantInvite              Permission = 1 << 0
	PermissionKickMembers                      Permission = 1 << 1
	PermissionBanMembers                       Permission = 1 << 2
	PermissionAdministrator                    Permission = 1 << 3
	PermissionManageChannels                   Permission = 1 << 4
	PermissionManageGuild                      Permission = 1 << 5
	PermissionAddReactions                     Permission = 1 << 6
	PermissionViewAuditLog                     Permission = 1 << 7
	PermissionPrioritySpeaker                  Permission = 1 << 8
	PermissionStream                           Permission = 1 << 9
	PermissionViewChannel                      Permission = 1 << 10
	PermissionSendMessages                     Permission = 1 << 11
	PermissionSendTtsMessages                  Permission = 1 << 12
	PermissionManageMessages                   Permission = 1 << 13
	PermissionEmbedLinks                       Permission = 1 << 14
	PermissionAttachFiles                      Permission = 1 << 15
	PermissionReadMessageHistory               Permission = 1 << 16
	PermissionMentionEveryone                  Permission = 1 << 17
	PermissionUseExternalEmojis                Permission = 1 << 18
	PermissionViewGuildInsights                Permission = 1 << 19
	PermissionConnect                          Permission = 1 << 20
	PermissionSpeak                            Permission = 1 << 21
	PermissionMuteMembers                      Permission = 1 << 22
	PermissionDeafenMembers                    Permission = 1 << 23
	PermissionMoveMembers                      Permission = 1 << 24
	PermissionUseVad                           Permission = 1 << 25
	PermissionChangeNickname                   Permission = 1 << 26
	PermissionManageNicknames                  Permission = 1 << 27
	PermissionManageRoles                      Permission = 1 << 28
	PermissionManageWebhooks                   Permission = 1 << 29
	PermissionManageGuildExpressions           Permission = 1 << 30
	PermissionUseApplicationCommands           Permission = 1 << 31
	PermissionRequestToSpeak                   Permission = 1 << 32
	PermissionManageEvents                     Permission = 1 << 33
	PermissionManageThreads                    Permission = 1 << 34
	PermissionCreatePublicThreads              Permission = 1 << 35
	PermissionCreatePrivateThreads             Permission = 1 << 36
	PermissionUseExternalStickers              Permission = 1 << 37
	PermissionSendMessagesInThreads            Permission = 1 << 38
	PermissionUseEmbeddedActivities            Permission = 1 << 39
	PermissionModerateMembers                  Permission = 1 << 40
	PermissionViewCreatorMonetizationAnalytics Permission = 1 << 41
	PermissionUseSoundboard                    Permission = 1 << 42
	PermissionCreateGuildExpressions           Permission = 1 << 43
	PermissionCreateEvents                     Permission = 1 << 44
	PermissionUseExternalSounds                Permission = 1 << 45
	PermissionSendVoiceMessages                Permission = 1 << 46
	PermissionSendPolls                        Permission = 1 << 49
	PermissionUseExternalApps                  Permission = 1 << 50
	PermissionPinMessages                      Permission = 1 << 51
	PermissionBypassSlowmode                   Permission = 1 << 52
)

// https://discord.com/developers/docs/topics/gateway-events#receive-events
type GatewayEventName string

const (
	EventReady             GatewayEventName = "READY"
	EventResumed           GatewayEventName = "RESUMED"
	EventGuildCreate       GatewayEventName = "GUILD_CREATE"
	EventGuildUpdate       GatewayEventName = "GUILD_UPDATE"
	EventGuildDelete       GatewayEventName = "GUILD_DELETE"
	EventVoiceStateUpdate  GatewayEventName = "VOICE_STATE_UPDATE"
	EventVoiceServerUpdate GatewayEventName = "VOICE_SERVER_UPDATE"
)
