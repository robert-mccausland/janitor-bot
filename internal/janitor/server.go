package janitor

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/robert-mccausland/janitor-bot/api/janitorv1"
	"github.com/robert-mccausland/janitor-bot/internal/discord"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// server serves the janitor's gRPC API to other bots on the Docker network.
type server struct {
	janitorv1.UnimplementedJanitorServer
	j *Janitor
}

// serveAPI listens on addr until the janitor's context ends.
func (j *Janitor) serveAPI(addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	s := grpc.NewServer()
	janitorv1.RegisterJanitorServer(s, &server{j: j})
	go func() {
		<-j.ctx.Done()
		s.GracefulStop()
	}()
	go func() {
		if err := s.Serve(lis); err != nil {
			logger.Error(fmt.Sprintf("gRPC server stopped: %v", err), slog.Any("err", err))
		}
	}()

	logger.Info("Serving the janitor API", slog.String("addr", addr))
	return nil
}

func (s *server) Identity(context.Context, *janitorv1.IdentityRequest) (*janitorv1.IdentityResponse, error) {
	user := s.j.client.User()
	if user == nil {
		return nil, status.Error(codes.Unavailable, "the janitor isn't connected to Discord")
	}
	return &janitorv1.IdentityResponse{UserId: user.ID}, nil
}

func (s *server) MoveMembers(ctx context.Context, req *janitorv1.MoveMembersRequest) (*janitorv1.MoveMembersResponse, error) {
	if len(req.GetUserIds()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "user_ids is required")
	}
	if req.GetGuildId() == "" || req.GetFromChannelId() == "" {
		return nil, status.Error(codes.InvalidArgument, "guild and from channel IDs are required")
	}
	if req.GetFromChannelId() == req.GetToChannelId() {
		return nil, status.Error(codes.InvalidArgument, "from and to are the same channel")
	}

	from, err := s.voiceChannel(req.GetGuildId(), req.GetFromChannelId())
	if err != nil {
		return nil, err
	}

	var to *discord.Channel
	if req.GetToChannelId() != "" {
		if to, err = s.voiceChannel(req.GetGuildId(), req.GetToChannelId()); err != nil {
			return nil, err
		}
	}

	only := map[string]bool{}
	for _, id := range req.GetUserIds() {
		only[id] = true
	}

	logger.Info("Moving members on request", slog.String("guild_id", req.GetGuildId()),
		slog.String("from_channel_id", from.ID), slog.String("to_channel_id", req.GetToChannelId()), slog.Int("requested", len(req.GetUserIds())))
	moved, err := moveSomeUsers(ctx, s.j.client, from, to, only, nil)
	if err != nil {
		if len(moved) == 0 && ctx.Err() != nil {
			return nil, status.FromContextError(ctx.Err()).Err()
		}
		logger.Error(fmt.Sprintf("error moving members on request: %v", err), slog.Any("err", err), slog.Int("moved", len(moved)))
		return &janitorv1.MoveMembersResponse{MovedUserIds: moved, Error: err.Error()}, nil
	}

	return &janitorv1.MoveMembersResponse{MovedUserIds: moved}, nil
}

func (s *server) HoldChannels(_ context.Context, req *janitorv1.HoldChannelsRequest) (*janitorv1.HoldChannelsResponse, error) {
	if req.GetGuildId() == "" {
		return nil, status.Error(codes.InvalidArgument, "guild_id is required")
	}
	if s.j.client.Guild(req.GetGuildId()) == nil {
		return nil, status.Errorf(codes.NotFound, "the janitor isn't in guild %s", req.GetGuildId())
	}

	holds := make(map[string]time.Time, len(req.GetChannels()))
	for _, h := range req.GetChannels() {
		holds[h.GetChannelId()] = time.Unix(h.GetUntilUnix(), 0)
	}
	s.j.holds.set(req.GetGuildId(), holds)

	return &janitorv1.HoldChannelsResponse{}, nil
}

// voiceChannel looks up a voice channel and checks it's in the guild.
func (s *server) voiceChannel(guildID, channelID string) (*discord.Channel, error) {
	if guildID == "" || channelID == "" {
		return nil, status.Error(codes.InvalidArgument, "guild and channel IDs are required")
	}

	channel, err := s.j.client.GetChannel(channelID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "channel %s: %v", channelID, err)
	}
	if channel.GuildID != guildID || channel.Type != discord.ChannelTypeGuildVoice {
		return nil, status.Errorf(codes.InvalidArgument, "channel %s isn't a voice channel in guild %s", channelID, guildID)
	}

	return channel, nil
}
