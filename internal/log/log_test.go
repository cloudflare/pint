package log_test

import (
	"log/slog"
	"testing"

	"github.com/cloudflare/pint/internal/log"

	"github.com/stretchr/testify/require"
)

func TestParseLevel(t *testing.T) {
	type testCaseT struct {
		s     string
		level slog.Level
		err   string
	}

	testCases := []testCaseT{
		{s: "xxx", level: slog.LevelInfo, err: `"xxx" is not a valid log level`},
		{s: "err", level: slog.LevelInfo, err: `"err" is not a valid log level`},
		{s: "DEB", level: slog.LevelInfo, err: `"DEB" is not a valid log level`},
		{s: "error", level: slog.LevelError},
		{s: "Error", level: slog.LevelError},
		{s: "ERROR", level: slog.LevelError},
		{s: "warn", level: slog.LevelWarn},
		{s: "Warn", level: slog.LevelWarn},
		{s: "WARN", level: slog.LevelWarn},
		{s: "info", level: slog.LevelInfo},
		{s: "Info", level: slog.LevelInfo},
		{s: "INFO", level: slog.LevelInfo},
		{s: "debug", level: slog.LevelDebug},
		{s: "Debug", level: slog.LevelDebug},
		{s: "DEBUG", level: slog.LevelDebug},
	}

	for _, tc := range testCases {
		t.Run(tc.s, func(t *testing.T) {
			l, err := log.ParseLevel(tc.s)
			if tc.err != "" {
				require.EqualError(t, err, tc.err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.level, l)
			}
		})
	}
}
