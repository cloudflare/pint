package reporter

import (
	"log/slog"
	"testing"

	"github.com/neilotoole/slogt"
)

func TestGitlabLogger(t *testing.T) {
	t.Run("Info logs message", func(t *testing.T) {
		slog.SetDefault(slogt.New(t))
		logger := gitlabLogger{}
		// This should not panic and should log
		logger.Info("test info message", "key1", "value1", "key2", 123)
	})

	t.Run("Warn logs message", func(t *testing.T) {
		slog.SetDefault(slogt.New(t))
		logger := gitlabLogger{}
		// This should not panic and should log
		logger.Warn("test warn message", "key1", "value1")
	})

	t.Run("Error logs message", func(t *testing.T) {
		slog.SetDefault(slogt.New(t))
		logger := gitlabLogger{}
		// This should not panic and should log
		logger.Error("test error message")
	})

	t.Run("Debug logs message", func(t *testing.T) {
		slog.SetDefault(slogt.New(t))
		logger := gitlabLogger{}
		// This should not panic and should log
		logger.Debug("test debug message", "key", "value")
	})
}
