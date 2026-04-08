package reporter

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/neilotoole/slogt"
	"github.com/stretchr/testify/require"
	"go.nhat.io/httpmock"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/discovery"
)

func TestBitBucketCommentAnchorIsEqual(t *testing.T) {
	type testCaseT struct {
		description string
		pending     BitBucketPendingCommentAnchor
		anchor      BitBucketCommentAnchor
		expected    bool
	}

	testCases := []testCaseT{
		{
			description: "all fields match",
			anchor: BitBucketCommentAnchor{
				Path:     "foo.yaml",
				Line:     10,
				LineType: "ADDED",
				DiffType: "EFFECTIVE",
			},
			pending: BitBucketPendingCommentAnchor{
				Path:     "foo.yaml",
				Line:     10,
				LineType: "ADDED",
				DiffType: "EFFECTIVE",
			},
			expected: true,
		},
		{
			description: "path mismatch",
			anchor: BitBucketCommentAnchor{
				Path:     "foo.yaml",
				Line:     10,
				LineType: "ADDED",
				DiffType: "EFFECTIVE",
			},
			pending: BitBucketPendingCommentAnchor{
				Path:     "bar.yaml",
				Line:     10,
				LineType: "ADDED",
				DiffType: "EFFECTIVE",
			},
			expected: false,
		},
		{
			description: "line mismatch",
			anchor: BitBucketCommentAnchor{
				Path:     "foo.yaml",
				Line:     10,
				LineType: "ADDED",
				DiffType: "EFFECTIVE",
			},
			pending: BitBucketPendingCommentAnchor{
				Path:     "foo.yaml",
				Line:     20,
				LineType: "ADDED",
				DiffType: "EFFECTIVE",
			},
			expected: false,
		},
		{
			description: "lineType mismatch",
			anchor: BitBucketCommentAnchor{
				Path:     "foo.yaml",
				Line:     10,
				LineType: "ADDED",
				DiffType: "EFFECTIVE",
			},
			pending: BitBucketPendingCommentAnchor{
				Path:     "foo.yaml",
				Line:     10,
				LineType: "CONTEXT",
				DiffType: "EFFECTIVE",
			},
			expected: false,
		},
		{
			description: "diffType mismatch",
			anchor: BitBucketCommentAnchor{
				Path:     "foo.yaml",
				Line:     10,
				LineType: "ADDED",
				DiffType: "EFFECTIVE",
			},
			pending: BitBucketPendingCommentAnchor{
				Path:     "foo.yaml",
				Line:     10,
				LineType: "ADDED",
				DiffType: "RANGE",
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			result := tc.anchor.isEqual(tc.pending)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestPendingCommentToBitBucketComment(t *testing.T) {
	type testCaseT struct {
		changes     *bitBucketPRChanges
		description string
		output      BitBucketPendingComment
		input       pendingComment
	}

	testCases := []testCaseT{
		{
			description: "nil changes",
			input: pendingComment{
				severity: "NORMAL",
				text:     "this is text",
				path:     "foo.yaml",
				line:     5,
			},
			output: BitBucketPendingComment{
				Text:     "this is text",
				Severity: "NORMAL",
				Anchor: BitBucketPendingCommentAnchor{
					Path:     "foo.yaml",
					Line:     5,
					DiffType: "EFFECTIVE",
					LineType: "CONTEXT",
					FileType: "FROM",
				},
			},
			changes: nil,
		},
		{
			description: "path not found in changes",
			input: pendingComment{
				severity: "NORMAL",
				text:     "this is text",
				path:     "foo.yaml",
				line:     5,
			},
			output: BitBucketPendingComment{
				Text:     "this is text",
				Severity: "NORMAL",
				Anchor: BitBucketPendingCommentAnchor{
					Path:     "foo.yaml",
					Line:     5,
					DiffType: "EFFECTIVE",
					LineType: "CONTEXT",
					FileType: "FROM",
				},
			},
			changes: &bitBucketPRChanges{
				pathModifiedLines: map[string][]int{"bar.yaml": {1, 2, 3}},
				pathLineMapping:   map[string]map[int]int{"bar.yaml": {1: 1, 2: 5, 3: 3}},
			},
		},
		{
			description: "path found in changes",
			input: pendingComment{
				severity: "NORMAL",
				text:     "this is text",
				path:     "foo.yaml",
				line:     5,
			},
			output: BitBucketPendingComment{
				Text:     "this is text",
				Severity: "NORMAL",
				Anchor: BitBucketPendingCommentAnchor{
					Path:     "foo.yaml",
					Line:     5,
					DiffType: "EFFECTIVE",
					LineType: "ADDED",
					FileType: "TO",
				},
			},
			changes: &bitBucketPRChanges{
				pathModifiedLines: map[string][]int{"foo.yaml": {1, 3, 5}},
				pathLineMapping:   map[string]map[int]int{"foo.yaml": {1: 1, 3: 3, 5: 4}},
			},
		},
		{
			description: "anchor before sets REMOVED lineType",
			input: pendingComment{
				severity: "NORMAL",
				text:     "this is text",
				path:     "foo.yaml",
				line:     5,
				anchor:   checks.AnchorBefore,
			},
			output: BitBucketPendingComment{
				Text:     "this is text",
				Severity: "NORMAL",
				Anchor: BitBucketPendingCommentAnchor{
					Path:     "foo.yaml",
					Line:     5,
					DiffType: "EFFECTIVE",
					LineType: "REMOVED",
					FileType: "FROM",
				},
			},
			changes: nil,
		},
		{
			description: "line not modified uses line mapping",
			input: pendingComment{
				severity: "NORMAL",
				text:     "this is text",
				path:     "foo.yaml",
				line:     5,
			},
			output: BitBucketPendingComment{
				Text:     "this is text",
				Severity: "NORMAL",
				Anchor: BitBucketPendingCommentAnchor{
					Path:     "foo.yaml",
					Line:     10,
					DiffType: "EFFECTIVE",
					LineType: "CONTEXT",
					FileType: "FROM",
				},
			},
			changes: &bitBucketPRChanges{
				pathModifiedLines: map[string][]int{"foo.yaml": {1, 3}},
				pathLineMapping:   map[string]map[int]int{"foo.yaml": {5: 10}},
			},
		},
		{
			description: "line not in mapping keeps original line",
			input: pendingComment{
				severity: "NORMAL",
				text:     "this is text",
				path:     "foo.yaml",
				line:     5,
			},
			output: BitBucketPendingComment{
				Text:     "this is text",
				Severity: "NORMAL",
				Anchor: BitBucketPendingCommentAnchor{
					Path:     "foo.yaml",
					Line:     5,
					DiffType: "EFFECTIVE",
					LineType: "CONTEXT",
					FileType: "FROM",
				},
			},
			changes: &bitBucketPRChanges{
				pathModifiedLines: map[string][]int{"foo.yaml": {1, 3}},
				pathLineMapping:   map[string]map[int]int{"foo.yaml": {1: 1, 3: 3}},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			slog.SetDefault(slogt.New(t))
			out := tc.input.toBitBucketComment(tc.changes)
			require.Equal(t, tc.output, out, "pendingComment.toBitBucketComment() returned wrong BitBucketPendingComment")
		})
	}
}

func TestReportToAnnotation(t *testing.T) {
	type testCaseT struct {
		description string
		output      BitBucketAnnotation
		input       Report
	}

	testCases := []testCaseT{
		{
			description: "fatal report on modified line",
			input: Report{
				Path: discovery.Path{
					SymlinkTarget: "foo.yaml",
					Name:          "foo.yaml",
				},
				ModifiedLines: []int{4, 5, 6},
				Problem: checks.Problem{
					Lines: diags.LineRange{
						First: 5,
						Last:  5,
					},
					Reporter: "mock",
					Summary:  "report text",
					Details:  "mock details",
					Severity: checks.Fatal,
				},
			},
			output: BitBucketAnnotation{
				Path:     "foo.yaml",
				Line:     5,
				Message:  "mock: report text",
				Severity: "HIGH",
				Type:     "BUG",
				Link:     "https://cloudflare.github.io/pint/checks/mock.html",
			},
		},
		{
			description: "bug report on modified line",
			input: Report{
				Path: discovery.Path{
					SymlinkTarget: "foo.yaml",
					Name:          "foo.yaml",
				},
				ModifiedLines: []int{4, 5, 6},
				Problem: checks.Problem{
					Lines: diags.LineRange{
						First: 5,
						Last:  5,
					},
					Reporter: "mock",
					Summary:  "report text",
					Severity: checks.Bug,
				},
			},
			output: BitBucketAnnotation{
				Path:     "foo.yaml",
				Line:     5,
				Message:  "mock: report text",
				Severity: "MEDIUM",
				Type:     "BUG",
				Link:     "https://cloudflare.github.io/pint/checks/mock.html",
			},
		},
		{
			description: "warning report on modified line",
			input: Report{
				Path: discovery.Path{
					SymlinkTarget: "foo.yaml",
					Name:          "foo.yaml",
				},
				ModifiedLines: []int{4, 5, 6},
				Problem: checks.Problem{
					Lines: diags.LineRange{
						First: 5,
						Last:  5,
					},
					Reporter: "mock",
					Summary:  "report text",
					Severity: checks.Warning,
				},
			},
			output: BitBucketAnnotation{
				Path:     "foo.yaml",
				Line:     5,
				Message:  "mock: report text",
				Severity: "LOW",
				Type:     "CODE_SMELL",
				Link:     "https://cloudflare.github.io/pint/checks/mock.html",
			},
		},
		{
			description: "information report on modified line",
			input: Report{
				Path: discovery.Path{
					SymlinkTarget: "foo.yaml",
					Name:          "foo.yaml",
				},
				ModifiedLines: []int{4, 5, 6},
				Problem: checks.Problem{
					Lines: diags.LineRange{
						First: 5,
						Last:  5,
					},
					Reporter: "mock",
					Summary:  "report text",
					Severity: checks.Information,
				},
			},
			output: BitBucketAnnotation{
				Path:     "foo.yaml",
				Line:     5,
				Message:  "mock: report text",
				Severity: "LOW",
				Type:     "CODE_SMELL",
				Link:     "https://cloudflare.github.io/pint/checks/mock.html",
			},
		},
		{
			description: "fatal report on symlinked file",
			input: Report{
				Path: discovery.Path{
					SymlinkTarget: "foo.yaml",
					Name:          "bar.yaml",
				},
				ModifiedLines: []int{4, 5, 6},
				Problem: checks.Problem{
					Lines: diags.LineRange{
						First: 5,
						Last:  5,
					},
					Reporter: "mock",
					Summary:  "report text",
					Severity: checks.Fatal,
				},
			},
			output: BitBucketAnnotation{
				Path:     "foo.yaml",
				Line:     5,
				Message:  "Problem detected on symlinked file bar.yaml: mock: report text",
				Severity: "HIGH",
				Type:     "BUG",
				Link:     "https://cloudflare.github.io/pint/checks/mock.html",
			},
		},
		{
			description: "fatal report on symlinked file on unmodified line",
			input: Report{
				Path: discovery.Path{
					SymlinkTarget: "foo.yaml",
					Name:          "bar.yaml",
				},
				ModifiedLines: []int{4, 5, 6},
				Problem: checks.Problem{
					Lines: diags.LineRange{
						First: 7,
						Last:  7,
					},
					Reporter: "mock",
					Summary:  "report text",
					Severity: checks.Fatal,
				},
			},
			output: BitBucketAnnotation{
				Path:     "foo.yaml",
				Line:     4,
				Message:  "Problem detected on symlinked file bar.yaml. Problem reported on unmodified line 7, annotation moved here: mock: report text",
				Severity: "HIGH",
				Type:     "BUG",
				Link:     "https://cloudflare.github.io/pint/checks/mock.html",
			},
		},
		{
			description: "information report on unmodified line",
			input: Report{
				Path: discovery.Path{
					SymlinkTarget: "foo.yaml",
					Name:          "foo.yaml",
				},
				ModifiedLines: []int{4, 5, 6},
				Problem: checks.Problem{
					Lines: diags.LineRange{
						First: 1,
						Last:  1,
					},
					Reporter: "mock",
					Summary:  "report text",
					Severity: checks.Information,
				},
			},
			output: BitBucketAnnotation{
				Path:     "foo.yaml",
				Line:     4,
				Message:  "Problem reported on unmodified line 1, annotation moved here: mock: report text",
				Severity: "LOW",
				Type:     "CODE_SMELL",
				Link:     "https://cloudflare.github.io/pint/checks/mock.html",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			slog.SetDefault(slogt.New(t))
			out := reportToAnnotation(tc.input)
			require.Equal(t, tc.output, out, "reportToAnnotation() returned wrong BitBucketAnnotation")
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

func TestBitBucketAPIPruneComments(t *testing.T) {
	slog.SetDefault(slogt.New(t))

	t.Run("keeps matching comment", func(t *testing.T) {
		// No requests should be made when comment matches.
		srv := httpmock.New(func(_ *httpmock.Server) {})(t)

		bb := bitBucketAPI{
			uri:       srv.URL(),
			authToken: "test-token",
			timeout:   time.Second * 5,
			project:   "proj",
			repo:      "repo",
		}

		pr := &bitBucketPR{ID: 1}
		currentComments := []bitBucketComment{
			{
				id:       1,
				version:  1,
				text:     "test comment",
				severity: "NORMAL",
				anchor: BitBucketCommentAnchor{
					Path:     "foo.yaml",
					Line:     10,
					LineType: "ADDED",
					DiffType: "EFFECTIVE",
				},
			},
		}
		pendingComments := []BitBucketPendingComment{
			{
				Text:     "test comment",
				Severity: "NORMAL",
				Anchor: BitBucketPendingCommentAnchor{
					Path:     "foo.yaml",
					Line:     10,
					LineType: "ADDED",
					DiffType: "EFFECTIVE",
				},
			},
		}

		bb.pruneComments(pr, currentComments, pendingComments)
		require.Empty(t, srv.Requests, "no requests should be made when comment matches")
	})

	t.Run("deletes comment with no replies", func(t *testing.T) {
		srv := httpmock.New(func(s *httpmock.Server) {
			s.ExpectDelete("/rest/api/1.0/projects/proj/repos/repo/pull-requests/1/comments/1?version=1").
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
		currentComments := []bitBucketComment{
			{
				id:       1,
				version:  1,
				text:     "old comment",
				severity: "NORMAL",
				replies:  0,
				anchor: BitBucketCommentAnchor{
					Path:     "foo.yaml",
					Line:     10,
					LineType: "ADDED",
					DiffType: "EFFECTIVE",
				},
			},
		}
		pendingComments := []BitBucketPendingComment{}

		bb.pruneComments(pr, currentComments, pendingComments)
		require.Len(t, srv.Requests, 1, "expected delete to be called")
		require.Equal(t, http.MethodDelete, srv.Requests[0].Method())
	})

	t.Run("resolves blocker comment with replies", func(t *testing.T) {
		srv := httpmock.New(func(s *httpmock.Server) {
			s.ExpectPut("/rest/api/1.0/projects/proj/repos/repo/pull-requests/1/comments/1").
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
		currentComments := []bitBucketComment{
			{
				id:       1,
				version:  1,
				text:     "old comment",
				severity: "BLOCKER",
				replies:  1,
				anchor: BitBucketCommentAnchor{
					Path:     "foo.yaml",
					Line:     10,
					LineType: "ADDED",
					DiffType: "EFFECTIVE",
				},
			},
		}
		pendingComments := []BitBucketPendingComment{}

		bb.pruneComments(pr, currentComments, pendingComments)
		require.Len(t, srv.Requests, 1, "expected resolve to be called")
		require.Equal(t, http.MethodPut, srv.Requests[0].Method())
	})

	t.Run("updates severity and resolves normal comment with replies", func(t *testing.T) {
		srv := httpmock.New(func(s *httpmock.Server) {
			// First PUT: severity update, second PUT: resolve.
			s.ExpectPut("/rest/api/1.0/projects/proj/repos/repo/pull-requests/1/comments/1").
				Once()
			s.ExpectPut("/rest/api/1.0/projects/proj/repos/repo/pull-requests/1/comments/1").
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
		currentComments := []bitBucketComment{
			{
				id:       1,
				version:  1,
				text:     "old comment",
				severity: "NORMAL",
				replies:  1,
				anchor: BitBucketCommentAnchor{
					Path:     "foo.yaml",
					Line:     10,
					LineType: "ADDED",
					DiffType: "EFFECTIVE",
				},
			},
		}
		pendingComments := []BitBucketPendingComment{}

		bb.pruneComments(pr, currentComments, pendingComments)
		require.Len(t, srv.Requests, 2, "expected severity update and resolve to be called")
		require.Equal(t, http.MethodPut, srv.Requests[0].Method())
		require.Equal(t, http.MethodPut, srv.Requests[1].Method())
	})

	t.Run("handles COMMIT diffType", func(t *testing.T) {
		srv := httpmock.New(func(s *httpmock.Server) {
			s.ExpectDelete("/rest/api/1.0/projects/proj/repos/repo/pull-requests/1/comments/1?version=1").
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
		currentComments := []bitBucketComment{
			{
				id:       1,
				version:  1,
				text:     "commit comment",
				severity: "NORMAL",
				replies:  0,
				anchor: BitBucketCommentAnchor{
					Path:     "foo.yaml",
					Line:     10,
					LineType: "ADDED",
					DiffType: "COMMIT",
				},
			},
		}
		pendingComments := []BitBucketPendingComment{
			{
				Text: "different comment",
				Anchor: BitBucketPendingCommentAnchor{
					Path:     "foo.yaml",
					Line:     10,
					LineType: "ADDED",
					DiffType: "EFFECTIVE",
				},
			},
		}

		bb.pruneComments(pr, currentComments, pendingComments)
		require.Len(t, srv.Requests, 1, "expected delete to be called for COMMIT diffType")
		require.Equal(t, http.MethodDelete, srv.Requests[0].Method())
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
				{ID: 1, Open: true, FromRef: BitBucketRef{ID: "refs/heads/other"}, ToRef: BitBucketRef{ID: "refs/heads/main"}},
			},
		})
		page2, _ := json.Marshal(BitBucketPullRequests{
			IsLastPage: true,
			Values: []BitBucketPullRequest{
				{ID: 2, Open: true, FromRef: BitBucketRef{ID: "refs/heads/feature", Commit: "abc123"}, ToRef: BitBucketRef{ID: "refs/heads/main", Commit: "def456"}},
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

func TestGetPullRequestChangesErrors(t *testing.T) {
	slog.SetDefault(slogt.New(t))

	t.Run("returns error on invalid JSON", func(t *testing.T) {
		srv := httpmock.New(func(s *httpmock.Server) {
			s.ExpectGet("/rest/api/1.0/projects/proj/repos/repo/pull-requests/1/changes?start=0").
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
		_, err := bb.getPullRequestChanges(pr)
		require.Error(t, err)
	})

	t.Run("returns error on getFileDiff failure", func(t *testing.T) {
		changesJSON, _ := json.Marshal(BitBucketPullRequestChanges{
			IsLastPage: true,
			Values:     []BitBucketPullRequestChange{{Path: BitBucketPath{ToString: "file.yaml"}}},
		})

		srv := httpmock.New(func(s *httpmock.Server) {
			s.ExpectGet("/rest/api/1.0/projects/proj/repos/repo/pull-requests/1/changes?start=0").
				ReturnHeader("Content-Type", "application/json").
				Return(string(changesJSON)).
				Once()
			s.ExpectGet("/rest/api/latest/projects/proj/repos/repo/commits/abc/diff/file.yaml?contextLines=10000&since=def&whitespace=show&withComments=false").
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

		pr := &bitBucketPR{ID: 1, srcHead: "abc", dstHead: "def"}
		_, err := bb.getPullRequestChanges(pr)
		require.Error(t, err)
	})

	t.Run("paginates through results", func(t *testing.T) {
		page1, _ := json.Marshal(BitBucketPullRequestChanges{
			IsLastPage:    false,
			NextPageStart: 1,
			Values:        []BitBucketPullRequestChange{},
		})
		page2, _ := json.Marshal(BitBucketPullRequestChanges{
			IsLastPage: true,
			Values:     []BitBucketPullRequestChange{},
		})

		srv := httpmock.New(func(s *httpmock.Server) {
			s.ExpectGet("/rest/api/1.0/projects/proj/repos/repo/pull-requests/1/changes?start=0").
				ReturnHeader("Content-Type", "application/json").
				Return(string(page1)).
				Once()
			s.ExpectGet("/rest/api/1.0/projects/proj/repos/repo/pull-requests/1/changes?start=1").
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

		pr := &bitBucketPR{ID: 1, srcHead: "abc", dstHead: "def"}
		_, err := bb.getPullRequestChanges(pr)
		require.NoError(t, err)
	})
}

func TestGetFileDiffErrors(t *testing.T) {
	slog.SetDefault(slogt.New(t))

	srv := httpmock.New(func(s *httpmock.Server) {
		s.ExpectGet("/rest/api/latest/projects/proj/repos/repo/commits/abc/diff/file.yaml?contextLines=10000&since=def&whitespace=show&withComments=false").
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

	pr := &bitBucketPR{ID: 1, srcHead: "abc", dstHead: "def"}
	_, _, err := bb.getFileDiff(pr, "file.yaml")
	require.Error(t, err)
}

func TestBitBucketAPIErrorHandling(t *testing.T) {
	slog.SetDefault(slogt.New(t))

	type testCaseT struct {
		run  func(bb bitBucketAPI, pr *bitBucketPR)
		name string
	}

	testCases := []testCaseT{
		{
			name: "updateSeverity logs error on failure",
			run: func(bb bitBucketAPI, pr *bitBucketPR) {
				cur := bitBucketComment{id: 1, version: 1, anchor: BitBucketCommentAnchor{Path: "file.yaml", Line: 10}}
				bb.updateSeverity(pr, cur, "BLOCKER")
			},
		},
		{
			name: "resolveTask logs error on failure",
			run: func(bb bitBucketAPI, pr *bitBucketPR) {
				cur := bitBucketComment{id: 1, version: 1}
				bb.resolveTask(pr, cur)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(_ *testing.T) {
			srv := httpmock.New(func(s *httpmock.Server) {
				s.ExpectPut("/rest/api/1.0/projects/proj/repos/repo/pull-requests/1/comments/1").
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
			tc.run(bb, pr)
		})
	}
}

func TestAddCommentsSkipsDuplicates(t *testing.T) {
	slog.SetDefault(slogt.New(t))

	srv := httpmock.New(func(s *httpmock.Server) {
		// Only the new comment should be posted, duplicate skipped.
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
	currentComments := []bitBucketComment{
		{
			id:   1,
			text: "existing comment",
			anchor: BitBucketCommentAnchor{
				Path:     "file.yaml",
				Line:     10,
				LineType: "ADDED",
				DiffType: "EFFECTIVE",
			},
		},
	}
	pendingComments := []BitBucketPendingComment{
		{
			Text: "existing comment",
			Anchor: BitBucketPendingCommentAnchor{
				Path:     "file.yaml",
				Line:     10,
				LineType: "ADDED",
				DiffType: "EFFECTIVE",
			},
		},
		{
			Text: "new comment",
			Anchor: BitBucketPendingCommentAnchor{
				Path:     "file.yaml",
				Line:     20,
				LineType: "ADDED",
				DiffType: "EFFECTIVE",
			},
		},
	}

	err := bb.addComments(pr, currentComments, pendingComments)
	require.NoError(t, err)
	require.Len(t, srv.Requests, 1, "only the new comment should be added, duplicate skipped")
	require.Equal(t, http.MethodPost, srv.Requests[0].Method())
}
