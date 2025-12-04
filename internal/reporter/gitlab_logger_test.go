package reporter

import (
	"errors"
	"log/slog"
	"testing"

	"github.com/neilotoole/slogt"
	"github.com/stretchr/testify/require"
	gitlab "gitlab.com/gitlab-org/api/client-go"
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

func TestDiffLineFor(t *testing.T) {
	testCases := []struct {
		name     string
		lines    []diffLine
		line     int64
		expected diffLine
		found    bool
	}{
		{
			name:     "empty lines slice",
			lines:    []diffLine{},
			line:     5,
			expected: diffLine{old: 0, new: 0, wasModified: false},
			found:    false,
		},
		{
			name: "exact match",
			lines: []diffLine{
				{old: 10, new: 10, wasModified: true},
				{old: 11, new: 12, wasModified: false},
			},
			line:     10,
			expected: diffLine{old: 10, new: 10, wasModified: true},
			found:    true,
		},
		{
			name: "line in gap - before first diff line",
			lines: []diffLine{
				{old: 10, new: 10, wasModified: true},
			},
			line:     5,
			expected: diffLine{old: 5, new: 5, wasModified: false},
			found:    true,
		},
		{
			name: "line in gap - between diff lines",
			lines: []diffLine{
				{old: 5, new: 5, wasModified: true},
				{old: 10, new: 12, wasModified: true},
			},
			line:     8,
			expected: diffLine{old: 8, new: 8, wasModified: false},
			found:    true,
		},
		{
			name: "line after all diff lines",
			lines: []diffLine{
				{old: 5, new: 5, wasModified: true},
				{old: 10, new: 10, wasModified: false},
			},
			line:     15,
			expected: diffLine{old: 15, new: 15, wasModified: false},
			found:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, found := diffLineFor(tc.lines, tc.line)
			require.Equal(t, tc.found, found)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestGetGitLabPaginated(t *testing.T) {
	t.Run("single page", func(t *testing.T) {
		items, err := getGitLabPaginated(func(_ int64) ([]string, *gitlab.Response, error) {
			return []string{"a", "b"}, &gitlab.Response{NextPage: 0}, nil
		})
		require.NoError(t, err)
		require.Equal(t, []string{"a", "b"}, items)
	})

	t.Run("multiple pages", func(t *testing.T) {
		callCount := 0
		items, err := getGitLabPaginated(func(pageNum int64) ([]string, *gitlab.Response, error) {
			callCount++
			if pageNum == 1 {
				return []string{"a"}, &gitlab.Response{NextPage: 2}, nil
			}
			return []string{"b"}, &gitlab.Response{NextPage: 0}, nil
		})
		require.NoError(t, err)
		require.Equal(t, []string{"a", "b"}, items)
		require.Equal(t, 2, callCount)
	})

	t.Run("error on first page", func(t *testing.T) {
		_, err := getGitLabPaginated(func(_ int64) ([]string, *gitlab.Response, error) {
			return nil, nil, errors.New("API error")
		})
		require.Error(t, err)
		require.Equal(t, "API error", err.Error())
	})
}

func TestLoggifyDiscussion(t *testing.T) {
	t.Run("nil position", func(t *testing.T) {
		opt := &gitlab.CreateMergeRequestDiscussionOptions{
			Position: nil,
		}
		attrs := loggifyDiscussion(opt)
		require.Nil(t, attrs)
	})

	t.Run("all position fields set", func(t *testing.T) {
		baseSHA := "base123"
		headSHA := "head456"
		startSHA := "start789"
		oldPath := "old/path.go"
		newPath := "new/path.go"
		var oldLine int64 = 10
		var newLine int64 = 20
		opt := &gitlab.CreateMergeRequestDiscussionOptions{
			Position: &gitlab.PositionOptions{
				BaseSHA:  &baseSHA,
				HeadSHA:  &headSHA,
				StartSHA: &startSHA,
				OldPath:  &oldPath,
				NewPath:  &newPath,
				OldLine:  &oldLine,
				NewLine:  &newLine,
			},
		}
		attrs := loggifyDiscussion(opt)
		require.Len(t, attrs, 7)
	})

	t.Run("partial position fields", func(t *testing.T) {
		baseSHA := "base123"
		opt := &gitlab.CreateMergeRequestDiscussionOptions{
			Position: &gitlab.PositionOptions{
				BaseSHA: &baseSHA,
			},
		}
		attrs := loggifyDiscussion(opt)
		require.Len(t, attrs, 1)
		require.Equal(t, "base_sha", attrs[0].Key)
		require.Equal(t, "base123", attrs[0].Value.String())
	})
}
