package discord

import (
	"encoding/json"
	"testing"
)

func TestNullableIDMarshalsEmptyAsNull(t *testing.T) {
	for _, tt := range []struct {
		id   nullableID
		want string
	}{
		{"", "null"},
		{"123", `"123"`},
	} {
		got, err := json.Marshal(tt.id)
		if err != nil || string(got) != tt.want {
			t.Errorf("Marshal(%q) = %s, %v; want %s", tt.id, got, err, tt.want)
		}
	}
}
