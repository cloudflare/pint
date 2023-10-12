package log

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

var Level = &slog.LevelVar{}

func Setup(level slog.Leveler, noColor bool) {
	Level.Set(level.Level())
	logger := slog.New(newHandler(os.Stderr, Level.Level(), noColor))
	slog.SetDefault(logger)
}

func ParseLevel(s string) (slog.Leveler, error) {
	switch strings.ToLower(s) {
	case "error":
		return slog.LevelError, nil
	case "warn":
		return slog.LevelWarn, nil
	case "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	default:
		return nil, fmt.Errorf("%q is not a valid log level", s)
	}
}
