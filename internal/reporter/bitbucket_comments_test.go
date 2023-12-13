package reporter

import (
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/neilotoole/slogt"

	"github.com/cloudflare/pint/internal/checks"
)

func TestBitBucketMakeComments(t *testing.T) {
	type testCaseT struct {
		description string
		summary     Summary
		changes     *bitBucketPRChanges
		comments    []BitBucketPendingComment
	}

	commentBody := func(icon, severity, reporter, text string) string {
		return fmt.Sprintf(
			":%s: **%s** reported by [pint](https://cloudflare.github.io/pint/) **%s** check.\n\n------\n\n%s\n\n------\n\n:information_source: To see documentation covering this check and instructions on how to resolve it [click here](https://cloudflare.github.io/pint/checks/%s.html).\n",
			icon, severity, reporter, text, reporter,
		)
	}

	testCases := []testCaseT{
		{
			description: "empty summary",
			comments:    []BitBucketPendingComment{},
		},
		{
			description: "report not included in changes",
			summary: Summary{reports: []Report{
				{
					ReportedPath:  "rule.yaml",
					SourcePath:    "rule.yaml",
					ModifiedLines: []int{2, 3},
					Problem: checks.Problem{
						Severity: checks.Bug,
					},
				},
			}},
			changes:  &bitBucketPRChanges{},
			comments: []BitBucketPendingComment{},
		},
		{
			description: "reports included in changes",
			summary: Summary{reports: []Report{
				{
					ReportedPath:  "rule.yaml",
					SourcePath:    "rule.yaml",
					ModifiedLines: []int{2, 3},
					Problem: checks.Problem{
						Severity: checks.Bug,
						Lines:    []int{2},
						Text:     "first error",
						Details:  "first details",
						Reporter: "r1",
					},
				},
				{
					ReportedPath:  "rule.yaml",
					SourcePath:    "rule.yaml",
					ModifiedLines: []int{2, 3},
					Problem: checks.Problem{
						Severity: checks.Warning,
						Lines:    []int{3},
						Text:     "second error",
						Details:  "second details",
						Reporter: "r1",
					},
				},
				{
					ReportedPath:  "rule.yaml",
					SourcePath:    "rule.yaml",
					ModifiedLines: []int{2, 3},
					Problem: checks.Problem{
						Severity: checks.Bug,
						Lines:    []int{2},
						Text:     "third error",
						Details:  "third details",
						Reporter: "r2",
					},
				},
				{
					ReportedPath:  "rule.yaml",
					SourcePath:    "symlink.yaml",
					ModifiedLines: []int{2, 3},
					Problem: checks.Problem{
						Severity: checks.Bug,
						Lines:    []int{2},
						Text:     "fourth error",
						Details:  "fourth details",
						Reporter: "r2",
					},
				},
				{
					ReportedPath:  "second.yaml",
					SourcePath:    "second.yaml",
					ModifiedLines: []int{1, 2, 3},
					Problem: checks.Problem{
						Anchor:   checks.AnchorBefore,
						Severity: checks.Bug,
						Lines:    []int{2},
						Text:     "fifth error",
						Details:  "fifth details",
						Reporter: "r2",
					},
				},
				{
					ReportedPath:  "second.yaml",
					SourcePath:    "second.yaml",
					ModifiedLines: []int{1, 2, 3},
					Problem: checks.Problem{
						Severity: checks.Bug,
						Lines:    []int{2},
						Text:     "sixth error",
						Details:  "sixth details",
						Reporter: "r2",
					},
				},
			}},
			changes: &bitBucketPRChanges{
				pathModifiedLines: map[string][]int{
					"rule.yaml":   {2, 3},
					"second.yaml": {1, 2, 3},
				},
				pathLineMapping: map[string]map[int]int{
					"rule.yaml":   {2: 2, 3: 3},
					"second.yaml": {1: 5, 2: 6, 3: 7},
				},
			},
			comments: []BitBucketPendingComment{
				{
					Text:     commentBody("stop_sign", "Bug", "r1", "first error\n\nfirst details"),
					Severity: "BLOCKER",
					Anchor: BitBucketPendingCommentAnchor{
						Path:     "rule.yaml",
						Line:     2,
						LineType: "ADDED",
						FileType: "TO",
						DiffType: "EFFECTIVE",
					},
				},
				{
					Text:     commentBody("warning", "Warning", "r1", "second error\n\nsecond details"),
					Severity: "NORMAL",
					Anchor: BitBucketPendingCommentAnchor{
						Path:     "rule.yaml",
						Line:     3,
						LineType: "ADDED",
						FileType: "TO",
						DiffType: "EFFECTIVE",
					},
				},
				{
					Text:     commentBody("stop_sign", "Bug", "r2", "third error\n\nthird details\n\n------\n\nfourth error\n\nfourth details\n\n:leftwards_arrow_with_hook: This problem was detected on a symlinked file `symlink.yaml`."),
					Severity: "BLOCKER",
					Anchor: BitBucketPendingCommentAnchor{
						Path:     "rule.yaml",
						Line:     2,
						LineType: "ADDED",
						FileType: "TO",
						DiffType: "EFFECTIVE",
					},
				},
				{
					Text:     commentBody("stop_sign", "Bug", "r2", "fifth error\n\nfifth details"),
					Severity: "BLOCKER",
					Anchor: BitBucketPendingCommentAnchor{
						Path:     "second.yaml",
						Line:     2,
						LineType: "REMOVED",
						FileType: "FROM",
						DiffType: "EFFECTIVE",
					},
				},
				{
					Text:     commentBody("stop_sign", "Bug", "r2", "sixth error\n\nsixth details"),
					Severity: "BLOCKER",
					Anchor: BitBucketPendingCommentAnchor{
						Path:     "second.yaml",
						Line:     2,
						LineType: "ADDED",
						FileType: "TO",
						DiffType: "EFFECTIVE",
					},
				},
			},
		},
		{
			description: "dedup reporter",
			summary: Summary{reports: []Report{
				{
					ReportedPath:  "rule.yaml",
					SourcePath:    "rule.yaml",
					ModifiedLines: []int{2, 3},
					Problem: checks.Problem{
						Severity: checks.Bug,
						Lines:    []int{2},
						Text:     "first error",
						Details:  "first details",
						Reporter: "r1",
					},
				},
				{
					ReportedPath:  "rule.yaml",
					SourcePath:    "rule.yaml",
					ModifiedLines: []int{2, 3},
					Problem: checks.Problem{
						Severity: checks.Bug,
						Lines:    []int{2},
						Text:     "second error",
						Details:  "second details",
						Reporter: "r1",
					},
				},
			}},
			changes: &bitBucketPRChanges{
				pathModifiedLines: map[string][]int{
					"rule.yaml": {2, 3},
				},
				pathLineMapping: map[string]map[int]int{
					"rule.yaml": {2: 2, 3: 3},
				},
			},
			comments: []BitBucketPendingComment{
				{
					Text:     commentBody("stop_sign", "Bug", "r1", "first error\n\nfirst details\n\n------\n\nsecond error\n\nsecond details"),
					Severity: "BLOCKER",
					Anchor: BitBucketPendingCommentAnchor{
						Path:     "rule.yaml",
						Line:     2,
						LineType: "ADDED",
						FileType: "TO",
						DiffType: "EFFECTIVE",
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			slog.SetDefault(slogt.New(t))
			r := NewBitBucketReporter(
				"v0.0.0",
				"http://bitbucket.example.com",
				time.Second,
				"token",
				"proj",
				"repo",
				nil)
			comments := r.api.makeComments(tc.summary, tc.changes)
			if diff := cmp.Diff(tc.comments, comments); diff != "" {
				t.Errorf("api.makeComments() returned wrong output (-want +got):\n%s", diff)
				return
			}
		})
	}
}
