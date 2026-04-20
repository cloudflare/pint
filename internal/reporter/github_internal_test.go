package reporter

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/neilotoole/slogt"
	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/git"
)

func TestGithubReporterDelete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	slog.SetDefault(slogt.New(t))
	r, err := NewGithubReporter(
		t.Context(),
		"v0.0.0",
		srv.URL,
		srv.URL,
		time.Second,
		"token",
		"owner",
		"repo",
		123,
		50,
		"HEAD",
		false,
	)
	require.NoError(t, err)

	// Delete should always return nil
	err = r.Delete(t.Context(), nil, ExistingComment{})
	require.NoError(t, err)
}

func TestGithubReporterIsEqual(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	slog.SetDefault(slogt.New(t))
	r, err := NewGithubReporter(
		t.Context(),
		"v0.0.0",
		srv.URL,
		srv.URL,
		time.Second,
		"token",
		"owner",
		"repo",
		123,
		50,
		"HEAD",
		false,
	)
	require.NoError(t, err)

	type testCaseT struct {
		name     string
		existing ExistingComment
		pending  PendingComment
		expected bool
	}

	testCases := []testCaseT{
		{
			name: "different paths",
			existing: ExistingComment{
				path: "file1.yml",
				line: 10,
				text: "comment",
			},
			pending: PendingComment{
				path: "file2.yml",
				line: 10,
				text: "comment",
			},
			expected: false,
		},
		{
			name: "same path, different line",
			existing: ExistingComment{
				path: "file.yml",
				line: 10,
				text: "comment",
			},
			pending: PendingComment{
				path: "file.yml",
				line: 20,
				text: "comment",
			},
			expected: false,
		},
		{
			name: "same path, different text",
			existing: ExistingComment{
				path: "file.yml",
				line: 10,
				text: "comment1",
			},
			pending: PendingComment{
				path: "file.yml",
				line: 10,
				text: "comment2",
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := r.IsEqual(nil, tc.existing, tc.pending)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestGithubReporterFixCommentLine(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	slog.SetDefault(slogt.New(t))
	r, err := NewGithubReporter(
		t.Context(),
		"v0.0.0",
		srv.URL,
		srv.URL,
		time.Second,
		"token",
		"owner",
		"repo",
		123,
		50,
		"HEAD",
		false,
	)
	require.NoError(t, err)

	testCases := []struct {
		name         string
		expectedSide string
		pending      PendingComment
		expectedLine int
	}{
		{
			// Modified line with AnchorBefore returns LEFT side and old line number.
			name: "anchor before returns LEFT side",
			pending: PendingComment{
				path:     "file.go",
				line:     2,
				anchor:   checks.AnchorBefore,
				lineMeta: git.LineMeta{Old: 1, Modified: true},
			},
			expectedSide: "LEFT",
			expectedLine: 1,
		},
		{
			// Modified line with AnchorAfter returns RIGHT side and current line number.
			name: "anchor after returns RIGHT side",
			pending: PendingComment{
				path:     "file.go",
				line:     2,
				anchor:   checks.AnchorAfter,
				lineMeta: git.LineMeta{Old: 1, Modified: true},
			},
			expectedSide: "RIGHT",
			expectedLine: 2,
		},
		{
			// Unmodified line returns RIGHT side and current line number.
			name: "unmodified line returns RIGHT side",
			pending: PendingComment{
				path:     "file.go",
				line:     100,
				anchor:   checks.AnchorAfter,
				lineMeta: git.LineMeta{Old: 100, Modified: false},
			},
			expectedSide: "RIGHT",
			expectedLine: 100,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			side, line := r.fixCommentLine(tc.pending)
			require.Equal(t, tc.expectedSide, side)
			require.Equal(t, tc.expectedLine, line)
		})
	}
}
