package reporter

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-github/v79/github"
	"github.com/neilotoole/slogt"
	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/checks"
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
			// Use empty ghPR for simple path/text comparison tests
			dst := ghPR{files: nil}
			result := r.IsEqual(dst, tc.existing, tc.pending)
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
		files        []*github.CommitFile
		pending      PendingComment
		expectedLine int
	}{
		{
			name: "anchor before returns LEFT side",
			files: []*github.CommitFile{
				{
					Filename: github.Ptr("file.go"),
					Patch:    github.Ptr("@@ -1,3 +1,4 @@\n line1\n+line2\n line3"),
				},
			},
			pending: PendingComment{
				path:   "file.go",
				line:   2,
				anchor: checks.AnchorBefore,
			},
			expectedSide: "LEFT",
			expectedLine: 1,
		},
		{
			name: "anchor after returns RIGHT side",
			files: []*github.CommitFile{
				{
					Filename: github.Ptr("file.go"),
					Patch:    github.Ptr("@@ -1,3 +1,4 @@\n line1\n+line2\n line3"),
				},
			},
			pending: PendingComment{
				path:   "file.go",
				line:   2,
				anchor: checks.AnchorAfter,
			},
			expectedSide: "RIGHT",
			expectedLine: 2,
		},
		{
			name: "unmodified line finds first modified",
			files: []*github.CommitFile{
				{
					Filename: github.Ptr("file.go"),
					Patch:    github.Ptr("@@ -1,3 +1,4 @@\n line1\n+line2\n line3"),
				},
			},
			pending: PendingComment{
				path:   "file.go",
				line:   100,
				anchor: checks.AnchorAfter,
			},
			expectedSide: "RIGHT",
			expectedLine: 2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dst := ghPR{files: tc.files}
			side, line := r.fixCommentLine(dst, tc.pending)
			require.Equal(t, tc.expectedSide, side)
			require.Equal(t, tc.expectedLine, line)
		})
	}
}
