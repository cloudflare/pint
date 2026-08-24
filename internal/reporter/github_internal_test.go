package reporter

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-github/v88/github"
	"github.com/neilotoole/slogt/v2"
	"github.com/stretchr/testify/require"
)

func TestOwnGitHubIssueComment(t *testing.T) {
	pintBody := signGeneralComment(tooManyCommentsMsg(3, 1))
	quoted := "> " + tooManyCommentsMsg(3, 1)
	author := github.User{ID: new(int64(42)), Login: new("ci-user")}
	other := github.User{ID: new(int64(99)), Login: new("reviewer")}

	require.True(t, ownGitHubIssueComment(42, &github.IssueComment{Body: new(pintBody), User: &author}))
	require.False(t, ownGitHubIssueComment(42, &github.IssueComment{Body: new(pintBody), User: &other}))
	require.False(t, ownGitHubIssueComment(42, &github.IssueComment{Body: new(quoted), User: &author}))
	require.False(t, ownGitHubIssueComment(42, &github.IssueComment{Body: new("LGTM"), User: &author}))
	require.True(t, ownGitHubIssueComment(0, &github.IssueComment{Body: new(pintBody), User: &other}))
	require.False(t, ownGitHubIssueComment(0, &github.IssueComment{Body: new("workflow status comment"), User: &author}))
}

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
			dst := ghPR{}
			result := r.IsEqual(dst, tc.existing, tc.pending)
			require.Equal(t, tc.expected, result)
		})
	}
}
