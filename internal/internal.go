package internal

import (
	"log/slog"

	"github.com/robert-mccausland/janitor-bot/internal/logging"
)

var logger *slog.Logger

func init() {
	logger = logging.NewLogger("github.com/robert-mccausland/janitor-bot/internal")
}
