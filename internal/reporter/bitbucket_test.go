package reporter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/neilotoole/slogt"
	"github.com/stretchr/testify/require"
	"go.nhat.io/httpmock"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/git"
)

func TestBitBucketReporterDescribe(t *testing.T) {
	// Verifies that Describe returns the reporter name.
	bb := BitBucketReporter{}
	require.Equal(t, "BitBucket", bb.Describe())
}

func TestBitBucketReporterDestinations(t *testing.T) {
	slog.SetDefault(slogt.New(t))

	// Verifies that Destinations returns an error when git HEAD fails.
	t.Run("git HEAD failure", func(t *testing.T) {
		bb := BitBucketReporter{
			api: newBitBucketAPI(
				"http://localhost", time.Second,
				"token", "proj", "repo",
			),
			gitCmd: func(args ...string) ([]byte, error) {
				if args[0] == "rev-parse" && args[1] == "--verify" {
					return nil, errors.New("git head error")
				}
				return nil, nil
			},
		}
		_, err := bb.Destinations(t.Context())
		require.EqualError(t, err, "failed to get HEAD commit: git head error")
	})

	// Verifies that Destinations returns an error when git branch fails.
	t.Run("git branch failure", func(t *testing.T) {
		bb := BitBucketReporter{
			api: newBitBucketAPI(
				"http://localhost", time.Second,
				"token", "proj", "repo",
			),
			gitCmd: func(args ...string) ([]byte, error) {
				if args[0] == "rev-parse" && args[1] == "--verify" {
					return []byte("abc123"), nil
				}
				if args[0] == "rev-parse" && args[1] == "--abbrev-ref" {
					return nil, errors.New("git branch error")
				}
				return nil, nil
			},
		}
		_, err := bb.Destinations(t.Context())
		require.EqualError(t, err, "failed to get current branch: git branch error")
	})

	// Verifies that Destinations returns nil when no PR matches.
	t.Run("no matching PR", func(t *testing.T) {
		prsJSON, _ := json.Marshal(BitBucketPullRequests{
			IsLastPage: true,
			Values:     []BitBucketPullRequest{},
		})

		srv := httpmock.New(func(s *httpmock.Server) {
			s.ExpectGet("/rest/api/1.0/projects/proj/repos/repo/commits/abc123/pull-requests?start=0").
				ReturnHeader("Content-Type", "application/json").
				Return(string(prsJSON)).
				Once()
		})(t)

		bb := BitBucketReporter{
			api: newBitBucketAPI(
				srv.URL(), time.Second,
				"token", "proj", "repo",
			),
			gitCmd: func(args ...string) ([]byte, error) {
				if args[0] == "rev-parse" && args[1] == "--verify" {
					return []byte("abc123"), nil
				}
				if args[0] == "rev-parse" && args[1] == "--abbrev-ref" {
					return []byte("feature"), nil
				}
				return nil, nil
			},
		}
		dsts, err := bb.Destinations(t.Context())
		require.NoError(t, err)
		require.Nil(t, dsts)
	})

	// Verifies that Destinations returns the matching PR destination.
	t.Run("matching PR found", func(t *testing.T) {
		prsJSON, _ := json.Marshal(BitBucketPullRequests{
			IsLastPage: true,
			Values: []BitBucketPullRequest{
				{
					ID:   42,
					Open: true,
					FromRef: BitBucketRef{
						ID:     "refs/heads/feature",
						Commit: "abc123",
					},
					ToRef: BitBucketRef{
						ID:     "refs/heads/main",
						Commit: "def456",
					},
				},
			},
		})

		srv := httpmock.New(func(s *httpmock.Server) {
			s.ExpectGet("/rest/api/1.0/projects/proj/repos/repo/commits/abc123/pull-requests?start=0").
				ReturnHeader("Content-Type", "application/json").
				Return(string(prsJSON)).
				Once()
		})(t)

		bb := BitBucketReporter{
			api: newBitBucketAPI(
				srv.URL(), time.Second,
				"token", "proj", "repo",
			),
			gitCmd: func(args ...string) ([]byte, error) {
				if args[0] == "rev-parse" && args[1] == "--verify" {
					return []byte("abc123"), nil
				}
				if args[0] == "rev-parse" && args[1] == "--abbrev-ref" {
					return []byte("feature"), nil
				}
				return nil, nil
			},
		}
		dsts, err := bb.Destinations(t.Context())
		require.NoError(t, err)
		require.Len(t, dsts, 1)
		pr := dsts[0].(*bitBucketPR)
		require.Equal(t, 42, pr.ID)
		require.Equal(t, "feature", pr.srcBranch)
		require.Equal(t, "abc123", pr.srcHead)
		require.Equal(t, "main", pr.dstBranch)
		require.Equal(t, "def456", pr.dstHead)
	})
}

func TestBitBucketReporterList(t *testing.T) {
	slog.SetDefault(slogt.New(t))

	// Verifies that List maps BitBucket comments to ExistingComment structs.
	t.Run("maps comments correctly", func(t *testing.T) {
		activitiesJSON, _ := json.Marshal(BitBucketPullRequestActivities{
			IsLastPage: true,
			Values: []BitBucketPullRequestActivity{
				{
					Action:        "COMMENTED",
					CommentAction: "ADDED",
					Comment: BitBucketPullRequestComment{
						ID:      10,
						Version: 2,
						State:   "OPEN",
						Author:  BitBucketCommentAuthor{Name: "testuser"},
						Text:    "some comment",
					},
					CommentAnchor: BitBucketCommentAnchor{
						Path: "foo.yaml",
						Line: 5,
					},
				},
			},
		})

		srv := httpmock.New(func(s *httpmock.Server) {
			s.ExpectGet("/plugins/servlet/applinks/whoami").
				Return("testuser").
				Once()
			s.ExpectGet("/rest/api/latest/projects/proj/repos/repo/pull-requests/1/activities?start=0").
				ReturnHeader("Content-Type", "application/json").
				Return(string(activitiesJSON)).
				Once()
		})(t)

		bb := BitBucketReporter{
			api: newBitBucketAPI(
				srv.URL(), time.Second,
				"token", "proj", "repo",
			),
		}
		pr := &bitBucketPR{ID: 1}
		existing, err := bb.List(t.Context(), pr)
		require.NoError(t, err)
		require.Len(t, existing, 1)
		require.Equal(t, "foo.yaml", existing[0].path)
		require.Equal(t, 5, existing[0].line)
		require.Equal(t, "some comment", existing[0].text)
		meta := existing[0].meta.(bitBucketCommentMeta)
		require.Equal(t, 10, meta.id)
		require.Equal(t, 2, meta.version)
	})
}

func TestBitBucketReporterCreate(t *testing.T) {
	slog.SetDefault(slogt.New(t))

	type testCaseT struct {
		description string
		wantSev     string
		wantAnchor  bitBucketPendingCommentAnchor
		pending     PendingComment
	}

	testCases := []testCaseT{
		{
			// Line is in changedLines (ADDED) so anchor should be ADDED/TO.
			description: "modified line uses ADDED/TO anchor",
			pending: PendingComment{
				path: "foo.yaml",
				line: 5,
				text: ":stop_sign: Bug found",
				changedLines: git.LineNumbers{
					{Before: 0, After: 5, Modified: true},
				},
			},
			wantAnchor: bitBucketPendingCommentAnchor{
				Path:     "foo.yaml",
				Line:     5,
				DiffType: "EFFECTIVE",
				LineType: "ADDED",
				FileType: "TO",
			},
			wantSev: "BLOCKER",
		},
		{
			// Line is NOT in changedLines so anchor should be CONTEXT/FROM.
			description: "unmodified line uses CONTEXT/FROM anchor",
			pending: PendingComment{
				path: "foo.yaml",
				line: 5,
				text: ":warning: Warning found",
				changedLines: git.LineNumbers{
					{Before: 0, After: 3, Modified: true},
				},
			},
			wantAnchor: bitBucketPendingCommentAnchor{
				Path:     "foo.yaml",
				Line:     5,
				DiffType: "EFFECTIVE",
				LineType: "CONTEXT",
				FileType: "FROM",
			},
			wantSev: "NORMAL",
		},
		{
			// AnchorBefore forces REMOVED lineType.
			description: "AnchorBefore uses REMOVED/FROM anchor",
			pending: PendingComment{
				path:   "foo.yaml",
				line:   5,
				text:   ":stop_sign: Bug found",
				anchor: checks.AnchorBefore,
				changedLines: git.LineNumbers{
					{Before: 0, After: 5, Modified: true},
				},
			},
			wantAnchor: bitBucketPendingCommentAnchor{
				Path:     "foo.yaml",
				Line:     5,
				DiffType: "EFFECTIVE",
				LineType: "REMOVED",
				FileType: "FROM",
			},
			wantSev: "BLOCKER",
		},
		{
			// Unmodified line with line mapping remaps the line number.
			description: "unmodified line uses BeforeForAfter mapping",
			pending: PendingComment{
				path: "foo.yaml",
				line: 5,
				text: ":warning: Warning found",
				changedLines: git.LineNumbers{
					{Before: 3, After: 5, Modified: false},
				},
			},
			wantAnchor: bitBucketPendingCommentAnchor{
				Path:     "foo.yaml",
				Line:     3,
				DiffType: "EFFECTIVE",
				LineType: "CONTEXT",
				FileType: "FROM",
			},
			wantSev: "NORMAL",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			srv := httpmock.New(func(s *httpmock.Server) {
				s.ExpectPost(
					"/rest/api/1.0/projects/proj/repos/repo/pull-requests/1/comments",
				).
					Return("{}").
					Once()
			})(t)

			bb := BitBucketReporter{
				api: newBitBucketAPI(
					srv.URL(), time.Second,
					"token", "proj", "repo",
				),
			}
			pr := &bitBucketPR{ID: 1}
			err := bb.Create(t.Context(), pr, tc.pending)
			require.NoError(t, err)
			require.Len(t, srv.Requests, 1)
			require.Equal(t, http.MethodPost, srv.Requests[0].Method())
		})
	}
}

func TestBitBucketReporterDelete(t *testing.T) {
	slog.SetDefault(slogt.New(t))

	// Verifies that Delete sends a DELETE request with correct comment ID and version.
	srv := httpmock.New(func(s *httpmock.Server) {
		s.ExpectDelete(
			"/rest/api/1.0/projects/proj/repos/repo/pull-requests/1/comments/10?version=2",
		).
			Once()
	})(t)

	bb := BitBucketReporter{
		api: newBitBucketAPI(
			srv.URL(), time.Second,
			"token", "proj", "repo",
		),
	}
	pr := &bitBucketPR{ID: 1}
	err := bb.Delete(t.Context(), pr, ExistingComment{
		path: "foo.yaml",
		line: 5,
		text: "old comment",
		meta: bitBucketCommentMeta{id: 10, version: 2},
	})
	require.NoError(t, err)
	require.Len(t, srv.Requests, 1)
	require.Equal(t, http.MethodDelete, srv.Requests[0].Method())
}

func TestBitBucketReporterCanCreate(t *testing.T) {
	type testCaseT struct {
		description string
		maxComments int
		done        int
		expected    bool
	}

	testCases := []testCaseT{
		{
			// Under the limit should allow creation.
			description: "under limit",
			maxComments: 10,
			done:        5,
			expected:    true,
		},
		{
			// At the limit should not allow creation.
			description: "at limit",
			maxComments: 10,
			done:        10,
			expected:    false,
		},
		{
			// Over the limit should not allow creation.
			description: "over limit",
			maxComments: 10,
			done:        15,
			expected:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			bb := BitBucketReporter{maxComments: tc.maxComments}
			require.Equal(t, tc.expected, bb.CanCreate(tc.done))
		})
	}
}

func TestBitBucketReporterCanDelete(t *testing.T) {
	// Verifies that CanDelete always returns true.
	bb := BitBucketReporter{}
	require.True(t, bb.CanDelete(ExistingComment{}))
}

func TestBitBucketReporterIsEqual(t *testing.T) {
	slog.SetDefault(slogt.New(t))

	type testCaseT struct {
		description string
		existing    ExistingComment
		pending     PendingComment
		expected    bool
	}

	testCases := []testCaseT{
		{
			// Same path, line, and text should be equal.
			description: "all fields match",
			existing: ExistingComment{
				path: "foo.yaml",
				line: 10,
				text: "comment text",
			},
			pending: PendingComment{
				path: "foo.yaml",
				line: 10,
				text: "comment text",
			},
			expected: true,
		},
		{
			// Different path should not be equal.
			description: "different path",
			existing: ExistingComment{
				path: "foo.yaml",
				line: 10,
				text: "comment text",
			},
			pending: PendingComment{
				path: "bar.yaml",
				line: 10,
				text: "comment text",
			},
			expected: false,
		},
		{
			// Different line should not be equal.
			description: "different line",
			existing: ExistingComment{
				path: "foo.yaml",
				line: 10,
				text: "comment text",
			},
			pending: PendingComment{
				path: "foo.yaml",
				line: 20,
				text: "comment text",
			},
			expected: false,
		},
		{
			// Different text should not be equal.
			description: "different text",
			existing: ExistingComment{
				path: "foo.yaml",
				line: 10,
				text: "comment A",
			},
			pending: PendingComment{
				path: "foo.yaml",
				line: 10,
				text: "comment B",
			},
			expected: false,
		},
		{
			// Trailing newline should be stripped before comparison.
			description: "trailing newline is ignored",
			existing: ExistingComment{
				path: "foo.yaml",
				line: 10,
				text: "comment text\n",
			},
			pending: PendingComment{
				path: "foo.yaml",
				line: 10,
				text: "comment text",
			},
			expected: true,
		},
	}

	bb := BitBucketReporter{}
	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			result := bb.IsEqual(nil, tc.existing, tc.pending)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestBitBucketAPIRequest(t *testing.T) {
	slog.SetDefault(slogt.New(t))

	t.Run("successful request with body", func(t *testing.T) {
		srv := httpmock.New(func(s *httpmock.Server) {
			s.ExpectPost("/test").
				WithHeader("Content-Type", "application/json").
				WithHeader("Authorization", "Bearer test-token").
				Return(`{"result": "ok"}`).
				Once()
		})(t)

		bb := bitBucketAPI{
			uri:       srv.URL(),
			authToken: "test-token",
			timeout:   time.Second * 5,
		}

		body := bytes.NewReader([]byte(`{"key": "value"}`))
		resp, err := bb.request(http.MethodPost, "/test", body)
		require.NoError(t, err)
		require.JSONEq(t, `{"result": "ok"}`, string(resp))
	})

	t.Run("request with invalid URL", func(t *testing.T) {
		bb := bitBucketAPI{
			uri:       "://invalid-url",
			authToken: "test-token",
			timeout:   time.Second,
		}

		_, err := bb.request(http.MethodGet, "/test", nil)
		require.Error(t, err)
	})

	t.Run("non-2xx response returns error", func(t *testing.T) {
		srv := httpmock.New(func(s *httpmock.Server) {
			s.ExpectGet("/test").
				ReturnCode(http.StatusBadRequest).
				Return("Bad Request").
				Once()
		})(t)

		bb := bitBucketAPI{
			uri:       srv.URL(),
			authToken: "test-token",
			timeout:   time.Second * 5,
		}

		_, err := bb.request(http.MethodGet, "/test", nil)
		require.Error(t, err)
		require.Equal(t, "GET request failed", err.Error())
	})
}

func TestBitBucketAPIGetPullRequestComments(t *testing.T) {
	slog.SetDefault(slogt.New(t))

	t.Run("filters comments correctly", func(t *testing.T) {
		activities := BitBucketPullRequestActivities{
			IsLastPage: true,
			Values: []BitBucketPullRequestActivity{
				{
					Action:        "COMMENTED",
					CommentAction: "ADDED",
					Comment: BitBucketPullRequestComment{
						ID:      1,
						Version: 1,
						State:   "OPEN",
						Author:  BitBucketCommentAuthor{Name: "testuser"},
						Text:    "valid comment",
					},
					CommentAnchor: BitBucketCommentAnchor{Path: "foo.yaml", Line: 10},
				},
				{
					Action:        "APPROVED",
					CommentAction: "ADDED",
					Comment: BitBucketPullRequestComment{
						ID:     2,
						State:  "OPEN",
						Author: BitBucketCommentAuthor{Name: "testuser"},
					},
				},
				{
					Action:        "COMMENTED",
					CommentAction: "EDITED",
					Comment: BitBucketPullRequestComment{
						ID:     3,
						State:  "OPEN",
						Author: BitBucketCommentAuthor{Name: "testuser"},
					},
				},
				{
					Action:        "COMMENTED",
					CommentAction: "ADDED",
					Comment: BitBucketPullRequestComment{
						ID:     4,
						State:  "RESOLVED",
						Author: BitBucketCommentAuthor{Name: "testuser"},
					},
				},
				{
					Action:        "COMMENTED",
					CommentAction: "ADDED",
					Comment: BitBucketPullRequestComment{
						ID:     5,
						State:  "OPEN",
						Author: BitBucketCommentAuthor{Name: "otheruser"},
					},
				},
				{
					Action:        "COMMENTED",
					CommentAction: "ADDED",
					Comment: BitBucketPullRequestComment{
						ID:       6,
						State:    "OPEN",
						Author:   BitBucketCommentAuthor{Name: "testuser"},
						Severity: "BLOCKER",
						Resolved: true,
					},
				},
				{
					Action:        "COMMENTED",
					CommentAction: "ADDED",
					Comment: BitBucketPullRequestComment{
						ID:       7,
						State:    "OPEN",
						Author:   BitBucketCommentAuthor{Name: "testuser"},
						Severity: "NORMAL",
					},
					CommentAnchor: BitBucketCommentAnchor{Orphaned: true},
				},
			},
		}
		activitiesJSON, _ := json.Marshal(activities)

		srv := httpmock.New(func(s *httpmock.Server) {
			s.ExpectGet("/plugins/servlet/applinks/whoami").
				Return("testuser").
				Once()
			s.ExpectGet("/rest/api/latest/projects/proj/repos/repo/pull-requests/1/activities?start=0").
				ReturnHeader("Content-Type", "application/json").
				Return(string(activitiesJSON)).
				Once()
		})(t)

		bb := bitBucketAPI{
			uri:       srv.URL(),
			authToken: "test-token",
			timeout:   time.Second * 5,
			project:   "proj",
			repo:      "repo",
		}

		pr := &bitBucketPR{ID: 1}
		comments, err := bb.getPullRequestComments(pr)
		require.NoError(t, err)
		require.Len(t, comments, 1)
		require.Equal(t, 1, comments[0].id)
		require.Equal(t, "valid comment", comments[0].text)
	})

	t.Run("handles pagination", func(t *testing.T) {
		page1, _ := json.Marshal(BitBucketPullRequestActivities{
			IsLastPage:    false,
			NextPageStart: 1,
			Values: []BitBucketPullRequestActivity{
				{
					Action:        "COMMENTED",
					CommentAction: "ADDED",
					Comment: BitBucketPullRequestComment{
						ID:     1,
						State:  "OPEN",
						Author: BitBucketCommentAuthor{Name: "testuser"},
						Text:   "comment 1",
					},
					CommentAnchor: BitBucketCommentAnchor{Path: "foo.yaml"},
				},
			},
		})
		page2, _ := json.Marshal(BitBucketPullRequestActivities{
			IsLastPage: true,
			Values: []BitBucketPullRequestActivity{
				{
					Action:        "COMMENTED",
					CommentAction: "ADDED",
					Comment: BitBucketPullRequestComment{
						ID:     2,
						State:  "OPEN",
						Author: BitBucketCommentAuthor{Name: "testuser"},
						Text:   "comment 2",
					},
					CommentAnchor: BitBucketCommentAnchor{Path: "bar.yaml"},
				},
			},
		})

		srv := httpmock.New(func(s *httpmock.Server) {
			s.ExpectGet("/plugins/servlet/applinks/whoami").
				Return("testuser").
				Once()
			s.ExpectGet("/rest/api/latest/projects/proj/repos/repo/pull-requests/1/activities?start=0").
				ReturnHeader("Content-Type", "application/json").
				Return(string(page1)).
				Once()
			s.ExpectGet("/rest/api/latest/projects/proj/repos/repo/pull-requests/1/activities?start=1").
				ReturnHeader("Content-Type", "application/json").
				Return(string(page2)).
				Once()
		})(t)

		bb := bitBucketAPI{
			uri:       srv.URL(),
			authToken: "test-token",
			timeout:   time.Second * 5,
			project:   "proj",
			repo:      "repo",
		}

		pr := &bitBucketPR{ID: 1}
		comments, err := bb.getPullRequestComments(pr)
		require.NoError(t, err)
		require.Len(t, comments, 2)
	})

	t.Run("returns error on invalid JSON", func(t *testing.T) {
		srv := httpmock.New(func(s *httpmock.Server) {
			s.ExpectGet("/plugins/servlet/applinks/whoami").
				Return("testuser").
				Once()
			s.ExpectGet("/rest/api/latest/projects/proj/repos/repo/pull-requests/1/activities?start=0").
				Return("invalid json").
				Once()
		})(t)

		bb := bitBucketAPI{
			uri:       srv.URL(),
			authToken: "test-token",
			timeout:   time.Second * 5,
			project:   "proj",
			repo:      "repo",
		}

		pr := &bitBucketPR{ID: 1}
		_, err := bb.getPullRequestComments(pr)
		require.Error(t, err)
	})
}

func TestFindPullRequestForBranchErrors(t *testing.T) {
	slog.SetDefault(slogt.New(t))

	t.Run("returns error on invalid JSON", func(t *testing.T) {
		srv := httpmock.New(func(s *httpmock.Server) {
			s.ExpectGet("/rest/api/1.0/projects/proj/repos/repo/commits/commit123/pull-requests?start=0").
				Return("invalid json").
				Once()
		})(t)

		bb := bitBucketAPI{
			uri:       srv.URL(),
			authToken: "test-token",
			timeout:   time.Second * 5,
			project:   "proj",
			repo:      "repo",
		}

		_, err := bb.findPullRequestForBranch("feature", "commit123")
		require.Error(t, err)
	})

	t.Run("paginates through results", func(t *testing.T) {
		page1, _ := json.Marshal(BitBucketPullRequests{
			IsLastPage:    false,
			NextPageStart: 1,
			Values: []BitBucketPullRequest{
				{
					ID:      1,
					Open:    true,
					FromRef: BitBucketRef{ID: "refs/heads/other"},
					ToRef:   BitBucketRef{ID: "refs/heads/main"},
				},
			},
		})
		page2, _ := json.Marshal(BitBucketPullRequests{
			IsLastPage: true,
			Values: []BitBucketPullRequest{
				{
					ID:      2,
					Open:    true,
					FromRef: BitBucketRef{ID: "refs/heads/feature", Commit: "abc123"},
					ToRef:   BitBucketRef{ID: "refs/heads/main", Commit: "def456"},
				},
			},
		})

		srv := httpmock.New(func(s *httpmock.Server) {
			s.ExpectGet("/rest/api/1.0/projects/proj/repos/repo/commits/commit123/pull-requests?start=0").
				ReturnHeader("Content-Type", "application/json").
				Return(string(page1)).
				Once()
			s.ExpectGet("/rest/api/1.0/projects/proj/repos/repo/commits/commit123/pull-requests?start=1").
				ReturnHeader("Content-Type", "application/json").
				Return(string(page2)).
				Once()
		})(t)

		bb := bitBucketAPI{
			uri:       srv.URL(),
			authToken: "test-token",
			timeout:   time.Second * 5,
			project:   "proj",
			repo:      "repo",
		}

		pr, err := bb.findPullRequestForBranch("feature", "commit123")
		require.NoError(t, err)
		require.NotNil(t, pr)
		require.Equal(t, 2, pr.ID)
	})
}

func TestBitBucketAPICreateComment(t *testing.T) {
	slog.SetDefault(slogt.New(t))

	// Verifies that createComment sends a POST with serialized comment body.
	t.Run("sends POST request with comment payload", func(t *testing.T) {
		srv := httpmock.New(func(s *httpmock.Server) {
			s.ExpectPost("/rest/api/1.0/projects/proj/repos/repo/pull-requests/1/comments").
				Return("{}").
				Once()
		})(t)

		bb := bitBucketAPI{
			uri:       srv.URL(),
			authToken: "test-token",
			timeout:   time.Second * 5,
			project:   "proj",
			repo:      "repo",
		}

		pr := &bitBucketPR{ID: 1}
		comment := BitBucketPendingComment{
			Text:     "test comment",
			Severity: "NORMAL",
			Anchor: bitBucketPendingCommentAnchor{
				Path:     "foo.yaml",
				Line:     10,
				DiffType: "EFFECTIVE",
				LineType: "ADDED",
				FileType: "TO",
			},
		}

		err := bb.createComment(pr, comment)
		require.NoError(t, err)
		require.Len(t, srv.Requests, 1)
		require.Equal(t, http.MethodPost, srv.Requests[0].Method())
	})

	// Verifies that createComment returns an error on server failure.
	t.Run("returns error on server failure", func(t *testing.T) {
		srv := httpmock.New(func(s *httpmock.Server) {
			s.ExpectPost("/rest/api/1.0/projects/proj/repos/repo/pull-requests/1/comments").
				ReturnCode(http.StatusInternalServerError).
				Once()
		})(t)

		bb := bitBucketAPI{
			uri:       srv.URL(),
			authToken: "test-token",
			timeout:   time.Second * 5,
			project:   "proj",
			repo:      "repo",
		}

		pr := &bitBucketPR{ID: 1}
		comment := BitBucketPendingComment{
			Text:     "test comment",
			Severity: "NORMAL",
		}

		err := bb.createComment(pr, comment)
		require.Error(t, err)
	})
}

func TestBitBucketAPIDeleteComment(t *testing.T) {
	slog.SetDefault(slogt.New(t))

	// Verifies that deleteComment sends a DELETE request with correct comment ID and version.
	t.Run("sends DELETE request with comment ID and version", func(t *testing.T) {
		srv := httpmock.New(func(s *httpmock.Server) {
			s.ExpectDelete("/rest/api/1.0/projects/proj/repos/repo/pull-requests/1/comments/42?version=3").
				Once()
		})(t)

		bb := bitBucketAPI{
			uri:       srv.URL(),
			authToken: "test-token",
			timeout:   time.Second * 5,
			project:   "proj",
			repo:      "repo",
		}

		pr := &bitBucketPR{ID: 1}
		err := bb.deleteComment(pr, 42, 3)
		require.NoError(t, err)
		require.Len(t, srv.Requests, 1)
		require.Equal(t, http.MethodDelete, srv.Requests[0].Method())
	})

	// Verifies that deleteComment returns an error on server failure.
	t.Run("returns error on server failure", func(t *testing.T) {
		srv := httpmock.New(func(s *httpmock.Server) {
			s.ExpectDelete("/rest/api/1.0/projects/proj/repos/repo/pull-requests/1/comments/42?version=3").
				ReturnCode(http.StatusInternalServerError).
				Once()
		})(t)

		bb := bitBucketAPI{
			uri:       srv.URL(),
			authToken: "test-token",
			timeout:   time.Second * 5,
			project:   "proj",
			repo:      "repo",
		}

		pr := &bitBucketPR{ID: 1}
		err := bb.deleteComment(pr, 42, 3)
		require.Error(t, err)
	})
}

func TestBitBucketAPIRequestDoError(t *testing.T) {
	slog.SetDefault(slogt.New(t))

	// Verifies that request returns error when HTTP client cannot connect.
	bb := bitBucketAPI{
		uri:       "http://127.0.0.1:0",
		authToken: "test-token",
		timeout:   time.Millisecond * 100,
	}

	_, err := bb.request(http.MethodGet, "/test", nil)
	require.Error(t, err)
}

func TestBitBucketAPIRequestReadBodyError(t *testing.T) {
	slog.SetDefault(slogt.New(t))

	// Verifies that request returns error when reading the response body fails.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(http.StatusOK)
		// Write fewer bytes than Content-Length then close, causing io.ReadAll to fail.
		_, _ = io.WriteString(w, "partial")
	}))
	t.Cleanup(srv.Close)

	bb := bitBucketAPI{
		uri:       srv.URL,
		authToken: "test-token",
		timeout:   time.Second,
	}

	_, err := bb.request(http.MethodGet, "/test", nil)
	require.Error(t, err)
}

func TestBitBucketAPIWhoamiError(t *testing.T) {
	slog.SetDefault(slogt.New(t))

	// Verifies that whoami returns error when the request fails.
	srv := httpmock.New(func(s *httpmock.Server) {
		s.ExpectGet("/plugins/servlet/applinks/whoami").
			ReturnCode(http.StatusInternalServerError).
			Once()
	})(t)

	bb := bitBucketAPI{
		uri:       srv.URL(),
		authToken: "test-token",
		timeout:   time.Second,
	}

	_, err := bb.whoami()
	require.EqualError(t, err, "GET request failed")
}

func TestFindPullRequestForBranchRequestError(t *testing.T) {
	slog.SetDefault(slogt.New(t))

	// Verifies that findPullRequestForBranch returns error when the HTTP request fails.
	srv := httpmock.New(func(s *httpmock.Server) {
		s.ExpectGet("/rest/api/1.0/projects/proj/repos/repo/commits/abc/pull-requests?start=0").
			ReturnCode(http.StatusInternalServerError).
			Once()
	})(t)

	bb := bitBucketAPI{
		uri:       srv.URL(),
		authToken: "test-token",
		timeout:   time.Second,
		project:   "proj",
		repo:      "repo",
	}

	_, err := bb.findPullRequestForBranch("feature", "abc")
	require.EqualError(t, err, "GET request failed")
}

func TestFindPullRequestForBranchSkipsClosedPR(t *testing.T) {
	slog.SetDefault(slogt.New(t))

	// Verifies that closed pull requests are skipped and nil is returned.
	prsJSON, _ := json.Marshal(BitBucketPullRequests{
		IsLastPage: true,
		Values: []BitBucketPullRequest{
			{
				ID:      1,
				Open:    false,
				FromRef: BitBucketRef{ID: "refs/heads/feature"},
				ToRef:   BitBucketRef{ID: "refs/heads/main"},
			},
		},
	})

	srv := httpmock.New(func(s *httpmock.Server) {
		s.ExpectGet("/rest/api/1.0/projects/proj/repos/repo/commits/abc/pull-requests?start=0").
			ReturnHeader("Content-Type", "application/json").
			Return(string(prsJSON)).
			Once()
	})(t)

	bb := bitBucketAPI{
		uri:       srv.URL(),
		authToken: "test-token",
		timeout:   time.Second,
		project:   "proj",
		repo:      "repo",
	}

	pr, err := bb.findPullRequestForBranch("feature", "abc")
	require.NoError(t, err)
	require.Nil(t, pr)
}

func TestGetPullRequestCommentsWhoamiError(t *testing.T) {
	slog.SetDefault(slogt.New(t))

	// Verifies that getPullRequestComments returns error when whoami fails.
	srv := httpmock.New(func(s *httpmock.Server) {
		s.ExpectGet("/plugins/servlet/applinks/whoami").
			ReturnCode(http.StatusInternalServerError).
			Once()
	})(t)

	bb := bitBucketAPI{
		uri:       srv.URL(),
		authToken: "test-token",
		timeout:   time.Second,
		project:   "proj",
		repo:      "repo",
	}

	pr := &bitBucketPR{ID: 1}
	_, err := bb.getPullRequestComments(pr)
	require.EqualError(t, err, "GET request failed")
}

func TestGetPullRequestCommentsActivitiesRequestError(t *testing.T) {
	slog.SetDefault(slogt.New(t))

	// Verifies that getPullRequestComments returns error when the activities request fails.
	srv := httpmock.New(func(s *httpmock.Server) {
		s.ExpectGet("/plugins/servlet/applinks/whoami").
			Return("testuser").
			Once()
		s.ExpectGet("/rest/api/latest/projects/proj/repos/repo/pull-requests/1/activities?start=0").
			ReturnCode(http.StatusInternalServerError).
			Once()
	})(t)

	bb := bitBucketAPI{
		uri:       srv.URL(),
		authToken: "test-token",
		timeout:   time.Second,
		project:   "proj",
		repo:      "repo",
	}

	pr := &bitBucketPR{ID: 1}
	_, err := bb.getPullRequestComments(pr)
	require.EqualError(t, err, "GET request failed")
}

func TestNewBitBucketReporter(t *testing.T) {
	slog.SetDefault(slogt.New(t))

	// Verifies that the constructor initializes all fields correctly.
	gitCmd := func(_ ...string) ([]byte, error) { return nil, nil }
	bb := NewBitBucketReporter(
		"http://localhost",
		time.Minute,
		"token",
		"proj",
		"repo",
		50,
		gitCmd,
	)
	require.Equal(t, "BitBucket", bb.Describe())
	require.Equal(t, 50, bb.maxComments)
	require.NotNil(t, bb.api)
	require.Equal(t, "http://localhost", bb.api.uri)
	require.Equal(t, "token", bb.api.authToken)
	require.Equal(t, "proj", bb.api.project)
	require.Equal(t, "repo", bb.api.repo)
	require.Equal(t, time.Minute, bb.api.timeout)
}

func TestBitBucketReporterDestinationsAPIError(t *testing.T) {
	slog.SetDefault(slogt.New(t))

	// Verifies that Destinations returns error when findPullRequestForBranch fails.
	srv := httpmock.New(func(s *httpmock.Server) {
		s.ExpectGet("/rest/api/1.0/projects/proj/repos/repo/commits/abc123/pull-requests?start=0").
			ReturnCode(http.StatusInternalServerError).
			Once()
	})(t)

	bb := BitBucketReporter{
		api: newBitBucketAPI(
			srv.URL(), time.Second,
			"token", "proj", "repo",
		),
		gitCmd: func(args ...string) ([]byte, error) {
			if args[0] == "rev-parse" && args[1] == "--verify" {
				return []byte("abc123"), nil
			}
			if args[0] == "rev-parse" && args[1] == "--abbrev-ref" {
				return []byte("feature"), nil
			}
			return nil, nil
		},
	}
	_, err := bb.Destinations(t.Context())
	require.ErrorContains(t, err, "failed to get open pull requests from BitBucket")
}

func TestBitBucketReporterSummary(t *testing.T) {
	// Verifies that Summary always returns nil.
	bb := BitBucketReporter{}
	err := bb.Summary(context.Background(), nil, Summary{}, nil, nil)
	require.NoError(t, err)
}

func TestBitBucketReporterListError(t *testing.T) {
	slog.SetDefault(slogt.New(t))

	// Verifies that List returns error when getPullRequestComments fails.
	srv := httpmock.New(func(s *httpmock.Server) {
		s.ExpectGet("/plugins/servlet/applinks/whoami").
			ReturnCode(http.StatusInternalServerError).
			Once()
	})(t)

	bb := BitBucketReporter{
		api: newBitBucketAPI(
			srv.URL(), time.Second,
			"token", "proj", "repo",
		),
	}
	pr := &bitBucketPR{ID: 1}
	_, err := bb.List(t.Context(), pr)
	require.EqualError(t, err, "GET request failed")
}
