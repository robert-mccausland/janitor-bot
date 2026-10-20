package janitor

import (
	"sync"
	"time"
)

// channelHolds are voice channels other bots have asked the janitor to leave
// alone (meeting rooms in use), per guild, each until a given time.
type channelHolds struct {
	mu sync.Mutex
	m  map[string]map[string]time.Time
}

// set replaces a guild's holds.
func (h *channelHolds) set(guildID string, holds map[string]time.Time) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.m == nil {
		h.m = map[string]map[string]time.Time{}
	}
	h.m[guildID] = holds
}

// held reports whether a channel is held at now.
func (h *channelHolds) held(guildID, channelID string, now time.Time) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	until, ok := h.m[guildID][channelID]
	return ok && now.Before(until)
}
