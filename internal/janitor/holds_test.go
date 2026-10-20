package janitor

import (
	"testing"
	"time"
)

func TestChannelHolds(t *testing.T) {
	now := time.Unix(1000, 0)
	h := &channelHolds{}
	if h.held("g", "room", now) {
		t.Fatal("nothing is held before any holds are set")
	}

	h.set("g", map[string]time.Time{"room": now.Add(time.Minute)})
	if !h.held("g", "room", now) {
		t.Error("room should be held before its hold lapses")
	}
	if h.held("g", "room", now.Add(time.Minute)) {
		t.Error("room should not be held once its hold lapses")
	}
	if h.held("other", "room", now) {
		t.Error("holds are per guild")
	}

	h.set("g", map[string]time.Time{})
	if h.held("g", "room", now) {
		t.Error("setting holds should replace the old ones")
	}
}
