package reporter_test

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/neilotoole/slogt/v2"
	"github.com/stretchr/testify/require"
	"go.nhat.io/httpmock"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/git"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/reporter"
)

func bbCommentText(severity, reporter, summary string) string {
	icon := ":stop_sign:"
	switch severity {
	case "Warning":
		icon = ":warning:"
	case "Information":
		icon = ":information_source:"
	}
	return icon + " **" + severity + "** reported by [pint](https://cloudflare.github.io/pint/) **" + reporter + "** check.\n\n------\n\n" + summary + "\n\n------\n\n:information_source: To see documentation covering this check and instructions on how to resolve it [click here](https://cloudflare.github.io/pint/checks/" + reporter + ".html).\n"
}

const (
	bbPRPath       = "/rest/api/1.0/projects/proj/repos/repo/commits/fake-commit-id/pull-requests"
	bbWhoami       = "/plugins/servlet/applinks/whoami"
	bbActivities   = "/rest/api/latest/projects/proj/repos/repo/pull-requests/1/activities"
	bbComments     = "/rest/api/1.0/projects/proj/repos/repo/pull-requests/1/comments"
	bbFakeBranch   = "fake-branch"
	bbFakeCommitID = "fake-commit-id"
)

func bbComment(id int) string {
	return fmt.Sprintf("%s/%d", bbComments, id)
}

func bbExpectPR(s *httpmock.Server) {
	s.ExpectGet(bbPRPath + "?start=0").ReturnJSON(reporter.BitBucketPullRequests{
		Values: []reporter.BitBucketPullRequest{
			{
				ID:   1,
				Open: true,
				FromRef: reporter.BitBucketRef{
					ID:     "refs/heads/fake-branch",
					Commit: "fake-commit-id",
				},
				ToRef: reporter.BitBucketRef{
					ID:     "refs/heads/main",
					Commit: "main-commit-id",
				},
			},
		},
		IsLastPage: true,
	})
}

func bbExpectWhoami(s *httpmock.Server) {
	s.ExpectGet(bbWhoami).Return("user")
}

func bbExpectEmptyActivities(s *httpmock.Server) {
	s.ExpectGet(bbActivities + "?start=0").ReturnJSON(
		reporter.BitBucketPullRequestActivities{IsLastPage: true},
	)
}

func TestBitBucketReporter(t *testing.T) {
	type testCaseT struct {
		mock           httpmock.Mocker
		errorHandler   func(err error) error
		setupSummary   func(*reporter.Summary)
		description    string
		reports        []reporter.Report
		maxComments    int
		showDuplicates bool
	}

	p := parser.NewParser(parser.DefaultOptions)
	mockFile := p.Parse(strings.NewReader(`
- record: target is down
  expr: up == 0
- record: sum errors
  expr: sum(errors) by (job)
`))

	for _, tc := range []testCaseT{
		{
			description: "no open pull request, no error",
			maxComments: 50,
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(bbPRPath + "?start=0").ReturnJSON(
					reporter.BitBucketPullRequests{IsLastPage: true},
				)
			}),
			reports: []reporter.Report{
				{
					Path: discovery.Path{
						Name:          "rule.yaml",
						SymlinkTarget: "rule.yaml",
					},
					Changes: &discovery.Changes{
						Lines: git.LineNumbers{{Before: 0, After: 2, Modified: true}},
					},
					Rule: mockFile.Groups[0].Rules[0],
					Problem: checks.Problem{
						Lines:    diags.LineRange{First: 2, Last: 2},
						Reporter: "mock",
						Summary:  "mock error",
						Severity: checks.Bug,
						Anchor:   checks.AnchorAfter,
					},
				},
			},
		},
		{
			description: "pull request found, comment created",
			maxComments: 50,
			mock: httpmock.New(func(s *httpmock.Server) {
				bbExpectPR(s)
				bbExpectWhoami(s)
				bbExpectEmptyActivities(s)
				s.ExpectPost(bbComments).
					WithBodyJSON(reporter.BitBucketPendingComment{
						Text:     bbCommentText("Bug", "mock", "mock error"),
						Severity: "NORMAL",
						Anchor: reporter.BitBucketPendingCommentAnchor{
							Path:     "rule.yaml",
							Line:     2,
							LineType: "ADDED",
							FileType: "TO",
							DiffType: "EFFECTIVE",
						},
					}).
					ReturnCode(http.StatusCreated)
			}),
			reports: []reporter.Report{
				{
					Path: discovery.Path{
						Name:          "rule.yaml",
						SymlinkTarget: "rule.yaml",
					},
					Changes: &discovery.Changes{
						Lines: git.LineNumbers{{Before: 0, After: 2, Modified: true}},
					},
					Rule: mockFile.Groups[0].Rules[0],
					Problem: checks.Problem{
						Lines:    diags.LineRange{First: 2, Last: 2},
						Reporter: "mock",
						Summary:  "mock error",
						Severity: checks.Bug,
						Anchor:   checks.AnchorAfter,
					},
				},
			},
		},
		{
			description: "fatal problem comment created",
			maxComments: 50,
			mock: httpmock.New(func(s *httpmock.Server) {
				bbExpectPR(s)
				bbExpectWhoami(s)
				bbExpectEmptyActivities(s)
				s.ExpectPost(bbComments).
					WithBodyJSON(reporter.BitBucketPendingComment{
						Text:     bbCommentText("Fatal", "mock", "fatal problem"),
						Severity: "NORMAL",
						Anchor: reporter.BitBucketPendingCommentAnchor{
							Path:     "rule.yaml",
							Line:     2,
							LineType: "ADDED",
							FileType: "TO",
							DiffType: "EFFECTIVE",
						},
					}).
					ReturnCode(http.StatusCreated)
			}),
			reports: []reporter.Report{
				{
					Path: discovery.Path{
						Name:          "rule.yaml",
						SymlinkTarget: "rule.yaml",
					},
					Changes: &discovery.Changes{
						Lines: git.LineNumbers{{Before: 0, After: 2, Modified: true}},
					},
					Rule: mockFile.Groups[0].Rules[0],
					Problem: checks.Problem{
						Lines:    diags.LineRange{First: 2, Last: 2},
						Reporter: "mock",
						Summary:  "fatal problem",
						Severity: checks.Fatal,
						Anchor:   checks.AnchorAfter,
					},
				},
			},
		},
		{
			description: "stale comment deleted",
			maxComments: 50,
			mock: httpmock.New(func(s *httpmock.Server) {
				bbExpectPR(s)
				bbExpectWhoami(s)
				s.ExpectGet(bbActivities + "?start=0").ReturnJSON(
					reporter.BitBucketPullRequestActivities{
						Values: []reporter.BitBucketPullRequestActivity{
							{
								Action:        "COMMENTED",
								CommentAction: "ADDED",
								Comment: reporter.BitBucketPullRequestComment{
									ID:       100,
									Version:  1,
									State:    "OPEN",
									Severity: "NORMAL",
									Text:     "stale comment text",
									Author:   reporter.BitBucketCommentAuthor{Name: "user"},
								},
								CommentAnchor: reporter.BitBucketCommentAnchor{
									Path: "old.yaml",
									Line: 5,
								},
							},
						},
						IsLastPage: true,
					},
				)
				s.ExpectPost(bbComments).
					WithBodyJSON(reporter.BitBucketPendingComment{
						Text:     bbCommentText("Warning", "mock", "new problem"),
						Severity: "NORMAL",
						Anchor: reporter.BitBucketPendingCommentAnchor{
							Path:     "rule.yaml",
							Line:     2,
							LineType: "ADDED",
							FileType: "TO",
							DiffType: "EFFECTIVE",
						},
					}).
					ReturnCode(http.StatusCreated)
				s.ExpectDelete(bbComment(100) + "?version=1")
			}),
			reports: []reporter.Report{
				{
					Path: discovery.Path{
						Name:          "rule.yaml",
						SymlinkTarget: "rule.yaml",
					},
					Changes: &discovery.Changes{
						Lines: git.LineNumbers{{Before: 0, After: 2, Modified: true}},
					},
					Rule: mockFile.Groups[0].Rules[0],
					Problem: checks.Problem{
						Lines:    diags.LineRange{First: 2, Last: 2},
						Reporter: "mock",
						Summary:  "new problem",
						Severity: checks.Warning,
						Anchor:   checks.AnchorAfter,
					},
				},
			},
		},
		{
			description: "comment with replies resolved instead of deleted",
			maxComments: 50,
			mock: httpmock.New(func(s *httpmock.Server) {
				bbExpectPR(s)
				bbExpectWhoami(s)
				s.ExpectGet(bbActivities + "?start=0").ReturnJSON(
					reporter.BitBucketPullRequestActivities{
						Values: []reporter.BitBucketPullRequestActivity{
							{
								Action:        "COMMENTED",
								CommentAction: "ADDED",
								Comment: reporter.BitBucketPullRequestComment{
									ID:       200,
									Version:  3,
									State:    "OPEN",
									Severity: "NORMAL",
									Text:     "stale comment with reply",
									Author:   reporter.BitBucketCommentAuthor{Name: "user"},
									Comments: []reporter.BitBucketPullRequestComment{
										{ID: 201, Text: "reply"},
									},
								},
								CommentAnchor: reporter.BitBucketCommentAnchor{
									Path: "old.yaml",
									Line: 5,
								},
							},
						},
						IsLastPage: true,
					},
				)
				s.ExpectPut(bbComment(200)).
					WithBodyJSON(reporter.BitBucketCommentSeverityUpdate{
						Severity: "BLOCKER",
						Version:  3,
					})
				s.ExpectPut(bbComment(200)).
					WithBodyJSON(reporter.BitBucketCommentStateUpdate{
						State:   "RESOLVED",
						Version: 3,
					})
			}),
		},
		{
			description: "whoami failure returns error",
			maxComments: 50,
			mock: httpmock.New(func(s *httpmock.Server) {
				bbExpectPR(s)
				s.ExpectGet(bbWhoami).ReturnCode(http.StatusInternalServerError)
			}),
			errorHandler: func(err error) error {
				if err != nil && err.Error() == "GET request failed" {
					return nil
				}
				return fmt.Errorf("unexpected error: %w", err)
			},
		},
		{
			description: "post comment failure returns error",
			maxComments: 50,
			mock: httpmock.New(func(s *httpmock.Server) {
				bbExpectPR(s)
				bbExpectWhoami(s)
				bbExpectEmptyActivities(s)
				s.ExpectPost(bbComments).ReturnCode(http.StatusInternalServerError)
			}),
			errorHandler: func(err error) error {
				if err != nil && err.Error() == "POST request failed" {
					return nil
				}
				return fmt.Errorf("unexpected error: %w", err)
			},
			reports: []reporter.Report{
				{
					Path: discovery.Path{
						Name:          "rule.yaml",
						SymlinkTarget: "rule.yaml",
					},
					Changes: &discovery.Changes{
						Lines: git.LineNumbers{{Before: 0, After: 2, Modified: true}},
					},
					Rule: mockFile.Groups[0].Rules[0],
					Problem: checks.Problem{
						Lines:    diags.LineRange{First: 2, Last: 2},
						Reporter: "mock",
						Summary:  "mock error",
						Severity: checks.Bug,
						Anchor:   checks.AnchorAfter,
					},
				},
			},
		},
		{
			description: "existing comment kept when content matches",
			maxComments: 50,
			mock: httpmock.New(func(s *httpmock.Server) {
				bbExpectPR(s)
				bbExpectWhoami(s)
				s.ExpectGet(bbActivities + "?start=0").ReturnJSON(
					reporter.BitBucketPullRequestActivities{
						Values: []reporter.BitBucketPullRequestActivity{
							{
								Action:        "COMMENTED",
								CommentAction: "ADDED",
								Comment: reporter.BitBucketPullRequestComment{
									ID:       300,
									Version:  1,
									State:    "OPEN",
									Severity: "NORMAL",
									Text:     bbCommentText("Warning", "mock", "this matches"),
									Author:   reporter.BitBucketCommentAuthor{Name: "user"},
								},
								CommentAnchor: reporter.BitBucketCommentAnchor{
									Path: "rule.yaml",
									Line: 2,
								},
							},
						},
						IsLastPage: true,
					},
				)
			}),
			reports: []reporter.Report{
				{
					Path: discovery.Path{
						Name:          "rule.yaml",
						SymlinkTarget: "rule.yaml",
					},
					Changes: &discovery.Changes{
						Lines: git.LineNumbers{{Before: 0, After: 2, Modified: true}},
					},
					Rule: mockFile.Groups[0].Rules[0],
					Problem: checks.Problem{
						Lines:    diags.LineRange{First: 2, Last: 2},
						Reporter: "mock",
						Summary:  "this matches",
						Severity: checks.Warning,
						Anchor:   checks.AnchorAfter,
					},
				},
			},
		},
		{
			description: "PR found on second page of results",
			maxComments: 50,
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(bbPRPath + "?start=0").ReturnJSON(reporter.BitBucketPullRequests{
					IsLastPage:    false,
					NextPageStart: 1,
					Values: []reporter.BitBucketPullRequest{
						{
							ID:      99,
							Open:    true,
							FromRef: reporter.BitBucketRef{ID: "refs/heads/other-branch"},
							ToRef:   reporter.BitBucketRef{ID: "refs/heads/main"},
						},
					},
				})
				s.ExpectGet(bbPRPath + "?start=1").ReturnJSON(reporter.BitBucketPullRequests{
					IsLastPage: true,
					Values: []reporter.BitBucketPullRequest{
						{
							ID:   1,
							Open: true,
							FromRef: reporter.BitBucketRef{
								ID:     "refs/heads/fake-branch",
								Commit: "fake-commit-id",
							},
							ToRef: reporter.BitBucketRef{
								ID:     "refs/heads/main",
								Commit: "main-commit-id",
							},
						},
					},
				})
				bbExpectWhoami(s)
				bbExpectEmptyActivities(s)
				s.ExpectPost(bbComments).
					WithBodyJSON(reporter.BitBucketPendingComment{
						Text:     bbCommentText("Bug", "mock", "mock error"),
						Severity: "NORMAL",
						Anchor: reporter.BitBucketPendingCommentAnchor{
							Path:     "rule.yaml",
							Line:     2,
							LineType: "ADDED",
							FileType: "TO",
							DiffType: "EFFECTIVE",
						},
					}).
					ReturnCode(http.StatusCreated)
			}),
			reports: []reporter.Report{
				{
					Path: discovery.Path{
						Name:          "rule.yaml",
						SymlinkTarget: "rule.yaml",
					},
					Changes: &discovery.Changes{
						Lines: git.LineNumbers{{Before: 0, After: 2, Modified: true}},
					},
					Rule: mockFile.Groups[0].Rules[0],
					Problem: checks.Problem{
						Lines:    diags.LineRange{First: 2, Last: 2},
						Reporter: "mock",
						Summary:  "mock error",
						Severity: checks.Bug,
						Anchor:   checks.AnchorAfter,
					},
				},
			},
		},
		{
			description: "comment filtering skips non-matching activities",
			maxComments: 50,
			mock: httpmock.New(func(s *httpmock.Server) {
				bbExpectPR(s)
				bbExpectWhoami(s)
				s.ExpectGet(bbActivities + "?start=0").ReturnJSON(
					reporter.BitBucketPullRequestActivities{
						Values: []reporter.BitBucketPullRequestActivity{
							{
								Action:        "APPROVED",
								CommentAction: "ADDED",
								Comment: reporter.BitBucketPullRequestComment{
									ID:     10,
									State:  "OPEN",
									Author: reporter.BitBucketCommentAuthor{Name: "user"},
								},
							},
							{
								Action:        "COMMENTED",
								CommentAction: "EDITED",
								Comment: reporter.BitBucketPullRequestComment{
									ID:     11,
									State:  "OPEN",
									Author: reporter.BitBucketCommentAuthor{Name: "user"},
								},
							},
							{
								Action:        "COMMENTED",
								CommentAction: "ADDED",
								Comment: reporter.BitBucketPullRequestComment{
									ID:     12,
									State:  "RESOLVED",
									Author: reporter.BitBucketCommentAuthor{Name: "user"},
								},
							},
							{
								Action:        "COMMENTED",
								CommentAction: "ADDED",
								Comment: reporter.BitBucketPullRequestComment{
									ID:     13,
									State:  "OPEN",
									Author: reporter.BitBucketCommentAuthor{Name: "otheruser"},
								},
							},
							{
								Action:        "COMMENTED",
								CommentAction: "ADDED",
								Comment: reporter.BitBucketPullRequestComment{
									ID:       14,
									State:    "OPEN",
									Author:   reporter.BitBucketCommentAuthor{Name: "user"},
									Severity: "BLOCKER",
									Resolved: true,
								},
							},
							{
								Action:        "COMMENTED",
								CommentAction: "ADDED",
								Comment: reporter.BitBucketPullRequestComment{
									ID:       15,
									State:    "OPEN",
									Author:   reporter.BitBucketCommentAuthor{Name: "user"},
									Severity: "NORMAL",
								},
								CommentAnchor: reporter.BitBucketCommentAnchor{Orphaned: true},
							},
						},
						IsLastPage: true,
					},
				)
				s.ExpectPost(bbComments).
					WithBodyJSON(reporter.BitBucketPendingComment{
						Text:     bbCommentText("Bug", "mock", "mock error"),
						Severity: "NORMAL",
						Anchor: reporter.BitBucketPendingCommentAnchor{
							Path:     "rule.yaml",
							Line:     2,
							LineType: "ADDED",
							FileType: "TO",
							DiffType: "EFFECTIVE",
						},
					}).
					ReturnCode(http.StatusCreated)
			}),
			reports: []reporter.Report{
				{
					Path: discovery.Path{
						Name:          "rule.yaml",
						SymlinkTarget: "rule.yaml",
					},
					Changes: &discovery.Changes{
						Lines: git.LineNumbers{{Before: 0, After: 2, Modified: true}},
					},
					Rule: mockFile.Groups[0].Rules[0],
					Problem: checks.Problem{
						Lines:    diags.LineRange{First: 2, Last: 2},
						Reporter: "mock",
						Summary:  "mock error",
						Severity: checks.Bug,
						Anchor:   checks.AnchorAfter,
					},
				},
			},
		},
		{
			description: "activities pagination across two pages",
			maxComments: 50,
			mock: httpmock.New(func(s *httpmock.Server) {
				bbExpectPR(s)
				bbExpectWhoami(s)
				s.ExpectGet(bbActivities + "?start=0").ReturnJSON(
					reporter.BitBucketPullRequestActivities{
						IsLastPage:    false,
						NextPageStart: 1,
						Values: []reporter.BitBucketPullRequestActivity{
							{
								Action:        "COMMENTED",
								CommentAction: "ADDED",
								Comment: reporter.BitBucketPullRequestComment{
									ID:       500,
									Version:  1,
									State:    "OPEN",
									Severity: "NORMAL",
									Text:     "stale page 1",
									Author:   reporter.BitBucketCommentAuthor{Name: "user"},
								},
								CommentAnchor: reporter.BitBucketCommentAnchor{
									Path: "old.yaml",
									Line: 5,
								},
							},
						},
					},
				)
				s.ExpectGet(bbActivities + "?start=1").ReturnJSON(
					reporter.BitBucketPullRequestActivities{
						IsLastPage: true,
						Values: []reporter.BitBucketPullRequestActivity{
							{
								Action:        "COMMENTED",
								CommentAction: "ADDED",
								Comment: reporter.BitBucketPullRequestComment{
									ID:       501,
									Version:  1,
									State:    "OPEN",
									Severity: "NORMAL",
									Text:     "stale page 2",
									Author:   reporter.BitBucketCommentAuthor{Name: "user"},
								},
								CommentAnchor: reporter.BitBucketCommentAnchor{
									Path: "old.yaml",
									Line: 10,
								},
							},
						},
					},
				)
				s.ExpectPost(bbComments).
					WithBodyJSON(reporter.BitBucketPendingComment{
						Text:     bbCommentText("Bug", "mock", "mock error"),
						Severity: "NORMAL",
						Anchor: reporter.BitBucketPendingCommentAnchor{
							Path:     "rule.yaml",
							Line:     2,
							LineType: "ADDED",
							FileType: "TO",
							DiffType: "EFFECTIVE",
						},
					}).
					ReturnCode(http.StatusCreated)
				s.ExpectDelete(bbComment(500) + "?version=1")
				s.ExpectDelete(bbComment(501) + "?version=1")
			}),
			reports: []reporter.Report{
				{
					Path: discovery.Path{
						Name:          "rule.yaml",
						SymlinkTarget: "rule.yaml",
					},
					Changes: &discovery.Changes{
						Lines: git.LineNumbers{{Before: 0, After: 2, Modified: true}},
					},
					Rule: mockFile.Groups[0].Rules[0],
					Problem: checks.Problem{
						Lines:    diags.LineRange{First: 2, Last: 2},
						Reporter: "mock",
						Summary:  "mock error",
						Severity: checks.Bug,
						Anchor:   checks.AnchorAfter,
					},
				},
			},
		},
		{
			description: "stale blocker comment with replies resolved directly",
			maxComments: 50,
			mock: httpmock.New(func(s *httpmock.Server) {
				bbExpectPR(s)
				bbExpectWhoami(s)
				s.ExpectGet(bbActivities + "?start=0").ReturnJSON(
					reporter.BitBucketPullRequestActivities{
						Values: []reporter.BitBucketPullRequestActivity{
							{
								Action:        "COMMENTED",
								CommentAction: "ADDED",
								Comment: reporter.BitBucketPullRequestComment{
									ID:       650,
									Version:  1,
									State:    "OPEN",
									Severity: "BLOCKER",
									Text:     "stale blocker with reply",
									Author:   reporter.BitBucketCommentAuthor{Name: "user"},
									Comments: []reporter.BitBucketPullRequestComment{
										{ID: 651, Text: "a reply"},
									},
								},
								CommentAnchor: reporter.BitBucketCommentAnchor{
									Path: "old.yaml",
									Line: 5,
								},
							},
						},
						IsLastPage: true,
					},
				)
				s.ExpectPost(bbComments).
					WithBodyJSON(reporter.BitBucketPendingComment{
						Text:     bbCommentText("Warning", "mock", "new problem"),
						Severity: "NORMAL",
						Anchor: reporter.BitBucketPendingCommentAnchor{
							Path:     "rule.yaml",
							Line:     2,
							LineType: "ADDED",
							FileType: "TO",
							DiffType: "EFFECTIVE",
						},
					}).
					ReturnCode(http.StatusCreated)
				s.ExpectPut(bbComment(650)).
					WithBodyJSON(reporter.BitBucketCommentStateUpdate{
						State:   "RESOLVED",
						Version: 1,
					})
			}),
			reports: []reporter.Report{
				{
					Path: discovery.Path{
						Name:          "rule.yaml",
						SymlinkTarget: "rule.yaml",
					},
					Changes: &discovery.Changes{
						Lines: git.LineNumbers{{Before: 0, After: 2, Modified: true}},
					},
					Rule: mockFile.Groups[0].Rules[0],
					Problem: checks.Problem{
						Lines:    diags.LineRange{First: 2, Last: 2},
						Reporter: "mock",
						Summary:  "new problem",
						Severity: checks.Warning,
						Anchor:   checks.AnchorAfter,
					},
				},
			},
		},
		{
			description: "stale comment with replies and non-BLOCKER severity gets upgraded then resolved",
			maxComments: 50,
			mock: httpmock.New(func(s *httpmock.Server) {
				bbExpectPR(s)
				bbExpectWhoami(s)
				s.ExpectGet(bbActivities + "?start=0").ReturnJSON(
					reporter.BitBucketPullRequestActivities{
						Values: []reporter.BitBucketPullRequestActivity{
							{
								Action:        "COMMENTED",
								CommentAction: "ADDED",
								Comment: reporter.BitBucketPullRequestComment{
									ID:       600,
									Version:  2,
									State:    "OPEN",
									Severity: "NORMAL",
									Text:     "stale non-blocker with reply",
									Author:   reporter.BitBucketCommentAuthor{Name: "user"},
									Comments: []reporter.BitBucketPullRequestComment{
										{ID: 601, Text: "a reply"},
									},
								},
								CommentAnchor: reporter.BitBucketCommentAnchor{
									Path: "old.yaml",
									Line: 5,
								},
							},
						},
						IsLastPage: true,
					},
				)
				s.ExpectPost(bbComments).
					WithBodyJSON(reporter.BitBucketPendingComment{
						Text:     bbCommentText("Warning", "mock", "new problem"),
						Severity: "NORMAL",
						Anchor: reporter.BitBucketPendingCommentAnchor{
							Path:     "rule.yaml",
							Line:     2,
							LineType: "ADDED",
							FileType: "TO",
							DiffType: "EFFECTIVE",
						},
					}).
					ReturnCode(http.StatusCreated)
				s.ExpectPut(bbComment(600)).
					WithBodyJSON(reporter.BitBucketCommentSeverityUpdate{
						Severity: "BLOCKER",
						Version:  2,
					})
				s.ExpectPut(bbComment(600)).
					WithBodyJSON(reporter.BitBucketCommentStateUpdate{
						State:   "RESOLVED",
						Version: 2,
					})
			}),
			reports: []reporter.Report{
				{
					Path: discovery.Path{
						Name:          "rule.yaml",
						SymlinkTarget: "rule.yaml",
					},
					Changes: &discovery.Changes{
						Lines: git.LineNumbers{{Before: 0, After: 2, Modified: true}},
					},
					Rule: mockFile.Groups[0].Rules[0],
					Problem: checks.Problem{
						Lines:    diags.LineRange{First: 2, Last: 2},
						Reporter: "mock",
						Summary:  "new problem",
						Severity: checks.Warning,
						Anchor:   checks.AnchorAfter,
					},
				},
			},
		},
		{
			description: "comment limit exceeded posts general comment",
			maxComments: 1,
			mock: httpmock.New(func(s *httpmock.Server) {
				bbExpectPR(s)
				bbExpectWhoami(s)
				bbExpectEmptyActivities(s)
				s.ExpectPost(bbComments).
					WithBodyJSON(reporter.BitBucketPendingComment{
						Text:     bbCommentText("Bug", "mock", "error one"),
						Severity: "NORMAL",
						Anchor: reporter.BitBucketPendingCommentAnchor{
							Path:     "rule.yaml",
							Line:     2,
							LineType: "ADDED",
							FileType: "TO",
							DiffType: "EFFECTIVE",
						},
					}).
					ReturnCode(http.StatusCreated)
				s.ExpectPost(bbComments).
					WithBodyJSON(reporter.BitBucketPendingComment{
						Text:     "This pint run would create 2 comment(s), which is more than the limit configured for pint (1).\n1 comment(s) were skipped and won't be visible on this PR.",
						Severity: "NORMAL",
						Anchor: reporter.BitBucketPendingCommentAnchor{
							DiffType: "EFFECTIVE",
						},
					}).
					ReturnCode(http.StatusCreated)
			}),
			reports: []reporter.Report{
				{
					Path: discovery.Path{
						Name:          "rule.yaml",
						SymlinkTarget: "rule.yaml",
					},
					Changes: &discovery.Changes{
						Lines: git.LineNumbers{{Before: 0, After: 2, Modified: true}},
					},
					Rule: mockFile.Groups[0].Rules[0],
					Problem: checks.Problem{
						Lines:    diags.LineRange{First: 2, Last: 2},
						Reporter: "mock",
						Summary:  "error one",
						Severity: checks.Bug,
						Anchor:   checks.AnchorAfter,
					},
				},
				{
					Path: discovery.Path{
						Name:          "rule.yaml",
						SymlinkTarget: "rule.yaml",
					},
					Changes: &discovery.Changes{
						Lines: git.LineNumbers{{Before: 0, After: 4, Modified: true}},
					},
					Rule: mockFile.Groups[0].Rules[1],
					Problem: checks.Problem{
						Lines:    diags.LineRange{First: 4, Last: 4},
						Reporter: "mock",
						Summary:  "error two",
						Severity: checks.Bug,
						Anchor:   checks.AnchorAfter,
					},
				},
			},
		},
		{
			description: "report on deleted line uses before anchor",
			maxComments: 50,
			mock: httpmock.New(func(s *httpmock.Server) {
				bbExpectPR(s)
				bbExpectWhoami(s)
				bbExpectEmptyActivities(s)
				s.ExpectPost(bbComments).
					WithBodyJSON(reporter.BitBucketPendingComment{
						Text:     bbCommentText("Warning", "mock", "deleted line problem"),
						Severity: "NORMAL",
						Anchor: reporter.BitBucketPendingCommentAnchor{
							Path:     "rule.yaml",
							Line:     2,
							LineType: "REMOVED",
							FileType: "FROM",
							DiffType: "EFFECTIVE",
						},
					}).
					ReturnCode(http.StatusCreated)
			}),
			reports: []reporter.Report{
				{
					Path: discovery.Path{
						Name:          "rule.yaml",
						SymlinkTarget: "rule.yaml",
					},
					Changes: &discovery.Changes{
						Lines: git.LineNumbers{
							{Before: 2, After: 0, Modified: true},
						},
					},
					Rule: mockFile.Groups[0].Rules[0],
					Problem: checks.Problem{
						Lines:    diags.LineRange{First: 2, Last: 2},
						Reporter: "mock",
						Summary:  "deleted line problem",
						Severity: checks.Warning,
						Anchor:   checks.AnchorBefore,
					},
				},
			},
		},
		{
			description: "report on context line uses old line number",
			maxComments: 50,
			mock: httpmock.New(func(s *httpmock.Server) {
				bbExpectPR(s)
				bbExpectWhoami(s)
				bbExpectEmptyActivities(s)
				s.ExpectPost(bbComments).
					WithBodyJSON(reporter.BitBucketPendingComment{
						Text:     bbCommentText("Warning", "mock", "context line problem"),
						Severity: "NORMAL",
						Anchor: reporter.BitBucketPendingCommentAnchor{
							Path:     "rule.yaml",
							Line:     3,
							LineType: "CONTEXT",
							FileType: "FROM",
							DiffType: "EFFECTIVE",
						},
					}).
					ReturnCode(http.StatusCreated)
			}),
			reports: []reporter.Report{
				{
					Path: discovery.Path{
						Name:          "rule.yaml",
						SymlinkTarget: "rule.yaml",
					},
					Changes: &discovery.Changes{
						Lines: git.LineNumbers{
							{Before: 3, After: 3, Modified: false},
						},
					},
					Rule: mockFile.Groups[0].Rules[0],
					Problem: checks.Problem{
						Lines:    diags.LineRange{First: 3, Last: 3},
						Reporter: "mock",
						Summary:  "context line problem",
						Severity: checks.Warning,
						Anchor:   checks.AnchorAfter,
					},
				},
			},
		},
		{
			description: "existing comment with same path and line but different text is replaced",
			maxComments: 50,
			mock: httpmock.New(func(s *httpmock.Server) {
				bbExpectPR(s)
				bbExpectWhoami(s)
				s.ExpectGet(bbActivities + "?start=0").ReturnJSON(
					reporter.BitBucketPullRequestActivities{
						Values: []reporter.BitBucketPullRequestActivity{
							{
								Action:        "COMMENTED",
								CommentAction: "ADDED",
								Comment: reporter.BitBucketPullRequestComment{
									ID:       800,
									Version:  1,
									State:    "OPEN",
									Severity: "NORMAL",
									Text:     "old text that no longer matches",
									Author:   reporter.BitBucketCommentAuthor{Name: "user"},
								},
								CommentAnchor: reporter.BitBucketCommentAnchor{
									Path: "rule.yaml",
									Line: 2,
								},
							},
							{
								Action:        "COMMENTED",
								CommentAction: "ADDED",
								Comment: reporter.BitBucketPullRequestComment{
									ID:       801,
									Version:  1,
									State:    "OPEN",
									Severity: "NORMAL",
									Text:     "comment on same path different line",
									Author:   reporter.BitBucketCommentAuthor{Name: "user"},
								},
								CommentAnchor: reporter.BitBucketCommentAnchor{
									Path: "rule.yaml",
									Line: 99,
								},
							},
						},
						IsLastPage: true,
					},
				)
				s.ExpectPost(bbComments).
					WithBodyJSON(reporter.BitBucketPendingComment{
						Text:     bbCommentText("Warning", "mock", "new text for same location"),
						Severity: "NORMAL",
						Anchor: reporter.BitBucketPendingCommentAnchor{
							Path:     "rule.yaml",
							Line:     2,
							LineType: "ADDED",
							FileType: "TO",
							DiffType: "EFFECTIVE",
						},
					}).
					ReturnCode(http.StatusCreated)
				s.ExpectDelete(bbComment(800) + "?version=1")
				s.ExpectDelete(bbComment(801) + "?version=1")
			}),
			reports: []reporter.Report{
				{
					Path: discovery.Path{
						Name:          "rule.yaml",
						SymlinkTarget: "rule.yaml",
					},
					Changes: &discovery.Changes{
						Lines: git.LineNumbers{{Before: 0, After: 2, Modified: true}},
					},
					Rule: mockFile.Groups[0].Rules[0],
					Problem: checks.Problem{
						Lines:    diags.LineRange{First: 2, Last: 2},
						Reporter: "mock",
						Summary:  "new text for same location",
						Severity: checks.Warning,
						Anchor:   checks.AnchorAfter,
					},
				},
			},
		},
		{
			description: "general comment failure on prometheus details",
			maxComments: 50,
			setupSummary: func(s *reporter.Summary) {
				s.MarkCheckDisabled("prom1", "config1", []string{"alerts/count"})
			},
			mock: httpmock.New(func(s *httpmock.Server) {
				bbExpectPR(s)
				bbExpectWhoami(s)
				bbExpectEmptyActivities(s)
				s.ExpectPost(bbComments).ReturnCode(http.StatusCreated)
				s.ExpectPost(bbComments).ReturnCode(http.StatusInternalServerError)
			}),
			errorHandler: func(err error) error {
				if err != nil && err.Error() == "failed to create general comment: POST request failed" {
					return nil
				}
				return fmt.Errorf("unexpected error: %w", err)
			},
			reports: []reporter.Report{
				{
					Path: discovery.Path{
						Name:          "rule.yaml",
						SymlinkTarget: "rule.yaml",
					},
					Changes: &discovery.Changes{
						Lines: git.LineNumbers{{Before: 0, After: 2, Modified: true}},
					},
					Rule: mockFile.Groups[0].Rules[0],
					Problem: checks.Problem{
						Lines:    diags.LineRange{First: 2, Last: 2},
						Reporter: "mock",
						Summary:  "some error",
						Severity: checks.Bug,
						Anchor:   checks.AnchorAfter,
					},
				},
			},
		},
		{
			description: "pull request lookup returns API error",
			maxComments: 50,
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(bbPRPath + "?start=0").ReturnCode(http.StatusInternalServerError)
			}),
			errorHandler: func(err error) error {
				if err != nil && err.Error() == "failed to get open pull requests from BitBucket: GET request failed" {
					return nil
				}
				return fmt.Errorf("unexpected error: %w", err)
			},
		},
		{
			description: "pull request lookup returns invalid JSON",
			maxComments: 50,
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(bbPRPath + "?start=0").Return("not json")
			}),
			errorHandler: func(err error) error {
				if err != nil && err.Error() == "failed to get open pull requests from BitBucket: invalid character 'o' in literal null (expecting 'u')" {
					return nil
				}
				return fmt.Errorf("unexpected error: %w", err)
			},
		},
		{
			description: "activities endpoint returns invalid JSON",
			maxComments: 50,
			mock: httpmock.New(func(s *httpmock.Server) {
				bbExpectPR(s)
				bbExpectWhoami(s)
				s.ExpectGet(bbActivities + "?start=0").Return("bad json")
			}),
			errorHandler: func(err error) error {
				if err != nil && err.Error() == "invalid character 'b' looking for beginning of value" {
					return nil
				}
				return fmt.Errorf("unexpected error: %w", err)
			},
			reports: []reporter.Report{
				{
					Path: discovery.Path{
						Name:          "rule.yaml",
						SymlinkTarget: "rule.yaml",
					},
					Changes: &discovery.Changes{
						Lines: git.LineNumbers{{Before: 0, After: 2, Modified: true}},
					},
					Rule: mockFile.Groups[0].Rules[0],
					Problem: checks.Problem{
						Lines:    diags.LineRange{First: 2, Last: 2},
						Reporter: "mock",
						Summary:  "problem",
						Severity: checks.Warning,
						Anchor:   checks.AnchorAfter,
					},
				},
			},
		},
		{
			description: "general comment failure on too many comments",
			maxComments: 1,
			mock: httpmock.New(func(s *httpmock.Server) {
				bbExpectPR(s)
				bbExpectWhoami(s)
				bbExpectEmptyActivities(s)
				s.ExpectPost(bbComments).ReturnCode(http.StatusCreated)
				s.ExpectPost(bbComments).Times(2).ReturnCode(http.StatusInternalServerError)
			}),
			errorHandler: func(err error) error {
				if err != nil && err.Error() == "failed to create general comment: POST request failed" {
					return nil
				}
				return fmt.Errorf("unexpected error: %w", err)
			},
			reports: []reporter.Report{
				{
					Path: discovery.Path{
						Name:          "rule.yaml",
						SymlinkTarget: "rule.yaml",
					},
					Changes: &discovery.Changes{
						Lines: git.LineNumbers{{Before: 0, After: 2, Modified: true}},
					},
					Rule: mockFile.Groups[0].Rules[0],
					Problem: checks.Problem{
						Lines:    diags.LineRange{First: 2, Last: 2},
						Reporter: "mock",
						Summary:  "error one",
						Severity: checks.Bug,
						Anchor:   checks.AnchorAfter,
					},
				},
				{
					Path: discovery.Path{
						Name:          "rule.yaml",
						SymlinkTarget: "rule.yaml",
					},
					Changes: &discovery.Changes{
						Lines: git.LineNumbers{{Before: 0, After: 4, Modified: true}},
					},
					Rule: mockFile.Groups[0].Rules[1],
					Problem: checks.Problem{
						Lines:    diags.LineRange{First: 4, Last: 4},
						Reporter: "mock",
						Summary:  "error two",
						Severity: checks.Bug,
						Anchor:   checks.AnchorAfter,
					},
				},
			},
		},
		{
			description: "severity update failure during stale comment removal",
			maxComments: 50,
			mock: httpmock.New(func(s *httpmock.Server) {
				bbExpectPR(s)
				bbExpectWhoami(s)
				s.ExpectGet(bbActivities + "?start=0").ReturnJSON(
					reporter.BitBucketPullRequestActivities{
						Values: []reporter.BitBucketPullRequestActivity{
							{
								Action:        "COMMENTED",
								CommentAction: "ADDED",
								Comment: reporter.BitBucketPullRequestComment{
									ID:       700,
									Version:  1,
									State:    "OPEN",
									Severity: "NORMAL",
									Text:     "stale non-blocker with reply",
									Author:   reporter.BitBucketCommentAuthor{Name: "user"},
									Comments: []reporter.BitBucketPullRequestComment{
										{ID: 701, Text: "reply"},
									},
								},
								CommentAnchor: reporter.BitBucketCommentAnchor{
									Path: "old.yaml",
									Line: 5,
								},
							},
						},
						IsLastPage: true,
					},
				)
				s.ExpectPost(bbComments).
					WithBodyJSON(reporter.BitBucketPendingComment{
						Text:     bbCommentText("Warning", "mock", "new problem"),
						Severity: "NORMAL",
						Anchor: reporter.BitBucketPendingCommentAnchor{
							Path:     "rule.yaml",
							Line:     2,
							LineType: "ADDED",
							FileType: "TO",
							DiffType: "EFFECTIVE",
						},
					}).
					ReturnCode(http.StatusCreated)
				s.ExpectPut(bbComment(700)).
					WithBodyJSON(reporter.BitBucketCommentSeverityUpdate{
						Severity: "BLOCKER",
						Version:  1,
					}).
					ReturnCode(http.StatusInternalServerError)
				s.ExpectPost(bbComments).ReturnCode(http.StatusInternalServerError)
			}),
			errorHandler: func(err error) error {
				if err != nil && err.Error() == "failed to create general comment: POST request failed" {
					return nil
				}
				return fmt.Errorf("unexpected error: %w", err)
			},
			reports: []reporter.Report{
				{
					Path: discovery.Path{
						Name:          "rule.yaml",
						SymlinkTarget: "rule.yaml",
					},
					Changes: &discovery.Changes{
						Lines: git.LineNumbers{{Before: 0, After: 2, Modified: true}},
					},
					Rule: mockFile.Groups[0].Rules[0],
					Problem: checks.Problem{
						Lines:    diags.LineRange{First: 2, Last: 2},
						Reporter: "mock",
						Summary:  "new problem",
						Severity: checks.Warning,
						Anchor:   checks.AnchorAfter,
					},
				},
			},
		},
		{
			description: "report on line outside diff becomes general comment",
			maxComments: 50,
			mock: httpmock.New(func(s *httpmock.Server) {
				bbExpectPR(s)
				bbExpectWhoami(s)
				bbExpectEmptyActivities(s)
				s.ExpectPost(bbComments).
					WithBodyJSON(reporter.BitBucketPendingComment{
						Text:     bbCommentText("Warning", "mock", "far away problem"),
						Severity: "NORMAL",
						Anchor: reporter.BitBucketPendingCommentAnchor{
							DiffType: "EFFECTIVE",
						},
					}).
					ReturnCode(http.StatusCreated)
			}),
			reports: []reporter.Report{
				{
					Path: discovery.Path{
						Name:          "rule.yaml",
						SymlinkTarget: "rule.yaml",
					},
					Changes: &discovery.Changes{
						Lines: git.LineNumbers{},
					},
					Rule: mockFile.Groups[0].Rules[0],
					Problem: checks.Problem{
						Lines:    diags.LineRange{First: 50, Last: 50},
						Reporter: "mock",
						Summary:  "far away problem",
						Severity: checks.Warning,
						Anchor:   checks.AnchorAfter,
					},
				},
			},
		},
		{
			description: "closed PR skipped in pagination",
			maxComments: 50,
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(bbPRPath + "?start=0").ReturnJSON(reporter.BitBucketPullRequests{
					Values: []reporter.BitBucketPullRequest{
						{
							ID:      1,
							Open:    false,
							FromRef: reporter.BitBucketRef{ID: "refs/heads/fake-branch"},
							ToRef:   reporter.BitBucketRef{ID: "refs/heads/main"},
						},
					},
					IsLastPage: true,
				})
			}),
			reports: []reporter.Report{
				{
					Path: discovery.Path{
						Name:          "rule.yaml",
						SymlinkTarget: "rule.yaml",
					},
					Changes: &discovery.Changes{
						Lines: git.LineNumbers{{Before: 0, After: 2, Modified: true}},
					},
					Rule: mockFile.Groups[0].Rules[0],
					Problem: checks.Problem{
						Lines:    diags.LineRange{First: 2, Last: 2},
						Reporter: "mock",
						Summary:  "mock error",
						Severity: checks.Bug,
						Anchor:   checks.AnchorAfter,
					},
				},
			},
		},
		{
			description: "activities request returns 500",
			maxComments: 50,
			mock: httpmock.New(func(s *httpmock.Server) {
				bbExpectPR(s)
				bbExpectWhoami(s)
				s.ExpectGet(bbActivities + "?start=0").ReturnCode(http.StatusInternalServerError)
			}),
			errorHandler: func(err error) error {
				if err != nil && err.Error() == "GET request failed" {
					return nil
				}
				return fmt.Errorf("unexpected error: %w", err)
			},
			reports: []reporter.Report{
				{
					Path: discovery.Path{
						Name:          "rule.yaml",
						SymlinkTarget: "rule.yaml",
					},
					Changes: &discovery.Changes{
						Lines: git.LineNumbers{{Before: 0, After: 2, Modified: true}},
					},
					Rule: mockFile.Groups[0].Rules[0],
					Problem: checks.Problem{
						Lines:    diags.LineRange{First: 2, Last: 2},
						Reporter: "mock",
						Summary:  "problem",
						Severity: checks.Warning,
						Anchor:   checks.AnchorAfter,
					},
				},
			},
		},
		{
			description: "comment creation fails with 500",
			maxComments: 50,
			mock: httpmock.New(func(s *httpmock.Server) {
				bbExpectPR(s)
				bbExpectWhoami(s)
				bbExpectEmptyActivities(s)
				s.ExpectPost(bbComments).ReturnCode(http.StatusInternalServerError)
			}),
			errorHandler: func(err error) error {
				if err != nil && err.Error() == "POST request failed" {
					return nil
				}
				return fmt.Errorf("unexpected error: %w", err)
			},
			reports: []reporter.Report{
				{
					Path: discovery.Path{
						Name:          "rule.yaml",
						SymlinkTarget: "rule.yaml",
					},
					Changes: &discovery.Changes{
						Lines: git.LineNumbers{{Before: 0, After: 2, Modified: true}},
					},
					Rule: mockFile.Groups[0].Rules[0],
					Problem: checks.Problem{
						Lines:    diags.LineRange{First: 2, Last: 2},
						Reporter: "mock",
						Summary:  "problem",
						Severity: checks.Warning,
						Anchor:   checks.AnchorAfter,
					},
				},
			},
		},
		{
			description: "exact duplicate reports are deduplicated",
			maxComments: 50,
			mock: httpmock.New(func(s *httpmock.Server) {
				bbExpectPR(s)
				bbExpectWhoami(s)
				bbExpectEmptyActivities(s)
				s.ExpectPost(bbComments).Times(3).ReturnCode(http.StatusCreated)
			}),
			reports: []reporter.Report{
				{
					Path: discovery.Path{
						Name:          "rule.yaml",
						SymlinkTarget: "rule.yaml",
					},
					Changes: &discovery.Changes{
						Lines: git.LineNumbers{{Before: 0, After: 2, Modified: true}},
					},
					Rule: mockFile.Groups[0].Rules[0],
					Problem: checks.Problem{
						Lines:    diags.LineRange{First: 2, Last: 2},
						Reporter: "mock",
						Summary:  "same problem",
						Details:  "same details",
						Severity: checks.Warning,
						Anchor:   checks.AnchorAfter,
						Diagnostics: []diags.Diagnostic{
							{Message: "diag A"},
						},
					},
				},
				{
					Path: discovery.Path{
						Name:          "rule.yaml",
						SymlinkTarget: "rule.yaml",
					},
					Changes: &discovery.Changes{
						Lines: git.LineNumbers{{Before: 0, After: 2, Modified: true}},
					},
					Rule: mockFile.Groups[0].Rules[0],
					Problem: checks.Problem{
						Lines:    diags.LineRange{First: 2, Last: 2},
						Reporter: "mock",
						Summary:  "same problem",
						Details:  "same details",
						Severity: checks.Warning,
						Anchor:   checks.AnchorAfter,
						Diagnostics: []diags.Diagnostic{
							{Message: "diag B"},
						},
					},
				},
				{
					Path: discovery.Path{
						Name:          "other.yaml",
						SymlinkTarget: "other.yaml",
					},
					Changes: &discovery.Changes{
						Lines: git.LineNumbers{{Before: 0, After: 2, Modified: true}},
					},
					Rule: mockFile.Groups[0].Rules[0],
					Problem: checks.Problem{
						Lines:    diags.LineRange{First: 2, Last: 2},
						Reporter: "mock",
						Summary:  "same problem",
						Details:  "same details",
						Severity: checks.Warning,
						Anchor:   checks.AnchorAfter,
						Diagnostics: []diags.Diagnostic{
							{Message: "diag C"},
						},
					},
				},
				{
					Path: discovery.Path{
						Name:          "rule.yaml",
						SymlinkTarget: "rule.yaml",
					},
					Changes: &discovery.Changes{
						Lines: git.LineNumbers{{Before: 0, After: 2, Modified: true}},
					},
					Rule: mockFile.Groups[0].Rules[0],
					Problem: checks.Problem{
						Lines:    diags.LineRange{First: 2, Last: 2},
						Reporter: "mock",
						Summary:  "same problem",
						Details:  "same details",
						Severity: checks.Warning,
						Anchor:   checks.AnchorBefore,
						Diagnostics: []diags.Diagnostic{
							{Message: "diag D"},
						},
					},
				},
			},
		},
	} {
		t.Run(tc.description, func(t *testing.T) {
			slog.SetDefault(slogt.New(t))

			srv := tc.mock(t)
			t.Cleanup(srv.Close)

			r := reporter.NewBitBucketReporter(
				srv.URL(),
				time.Second,
				"token",
				"proj",
				"repo",
				bbFakeBranch,
				bbFakeCommitID,
				tc.maxComments,
			)

			summary := reporter.NewSummary(tc.reports)
			summary.SortReports()
			summary.Dedup()
			if tc.setupSummary != nil {
				tc.setupSummary(&summary)
			}
			err := reporter.Submit(t.Context(), summary, r, tc.showDuplicates)

			if tc.errorHandler != nil {
				require.NoError(t, tc.errorHandler(err))
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestBitBucketReporterInvalidURI(t *testing.T) {
	slog.SetDefault(slogt.New(t))

	r := reporter.NewBitBucketReporter(
		"http://\x01",
		time.Second,
		"token",
		"proj",
		"repo",
		bbFakeBranch,
		bbFakeCommitID,
		50,
	)

	summary := reporter.NewSummary(nil)
	err := reporter.Submit(t.Context(), summary, r, false)
	require.EqualError(
		t, err,
		`failed to get open pull requests from BitBucket: parse "http://\x01/rest/api/1.0/projects/proj/repos/repo/commits/fake-commit-id/pull-requests?start=0": net/url: invalid control character in URL`,
	)
}

func TestBitBucketReporterConnectionRefused(t *testing.T) {
	slog.SetDefault(slogt.New(t))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	closedURL := srv.URL
	srv.Close()

	r := reporter.NewBitBucketReporter(
		closedURL,
		time.Second,
		"token",
		"proj",
		"repo",
		bbFakeBranch,
		bbFakeCommitID,
		50,
	)

	summary := reporter.NewSummary(nil)
	err := reporter.Submit(t.Context(), summary, r, false)
	require.Error(t, err)
}

func TestBitBucketReporterTruncatedResponseBody(t *testing.T) {
	slog.SetDefault(slogt.New(t))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/pull-requests") {
			w.Header().Set("Content-Length", "1000")
			_, _ = w.Write([]byte(`{`))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	r := reporter.NewBitBucketReporter(
		srv.URL,
		time.Second,
		"token",
		"proj",
		"repo",
		bbFakeBranch,
		bbFakeCommitID,
		50,
	)

	summary := reporter.NewSummary(nil)
	err := reporter.Submit(t.Context(), summary, r, false)
	require.EqualError(t, err, "failed to get open pull requests from BitBucket: unexpected EOF")
}

func TestBitBucketReporterRequestValidatesToken(t *testing.T) {
	slog.SetDefault(slogt.New(t))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = fmt.Fprintln(w, "bad token")
			return
		}

		switch {
		case strings.Contains(r.URL.Path, "/pull-requests"):
			noPRs, _ := json.Marshal(reporter.BitBucketPullRequests{IsLastPage: true})
			_, _ = w.Write(noPRs)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	t.Cleanup(srv.Close)

	r := reporter.NewBitBucketReporter(
		srv.URL,
		time.Second,
		"test-token",
		"proj",
		"repo",
		"branch",
		"abc123",
		50,
	)

	summary := reporter.NewSummary(nil)
	err := reporter.Submit(t.Context(), summary, r, false)
	require.NoError(t, err)
}
