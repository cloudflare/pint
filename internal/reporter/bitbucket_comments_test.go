package reporter

import (
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/neilotoole/slogt"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/discovery"
)

func TestBitBucketMakeComments(t *testing.T) {
	type testCaseT struct {
		changes        *bitBucketPRChanges
		description    string
		comments       []BitBucketPendingComment
		summary        Summary
		maxComments    int
		showDuplicates bool
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
			maxComments: 50,
			comments:    []BitBucketPendingComment{},
		},
		{
			description: "report not included in changes",
			maxComments: 50,
			summary: Summary{reports: []Report{
				{
					Path: discovery.Path{
						SymlinkTarget: "rule.yaml",
						Name:          "rule.yaml",
					},
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
			maxComments: 50,
			summary: Summary{reports: []Report{
				{
					Path: discovery.Path{
						SymlinkTarget: "rule.yaml",
						Name:          "rule.yaml",
					},
					ModifiedLines: []int{2, 3},
					Problem: checks.Problem{
						Severity: checks.Bug,
						Lines: diags.LineRange{
							First: 2,
							Last:  2,
						},
						Summary:  "first error",
						Details:  "first details",
						Reporter: "r1",
					},
				},
				{
					Path: discovery.Path{
						SymlinkTarget: "rule.yaml",
						Name:          "rule.yaml",
					},
					ModifiedLines: []int{2, 3},
					Problem: checks.Problem{
						Severity: checks.Warning,
						Lines: diags.LineRange{
							First: 3,
							Last:  3,
						},
						Summary:  "second error",
						Details:  "second details",
						Reporter: "r1",
					},
				},
				{
					Path: discovery.Path{
						SymlinkTarget: "rule.yaml",
						Name:          "rule.yaml",
					},
					ModifiedLines: []int{2, 3},
					Problem: checks.Problem{
						Severity: checks.Bug,
						Lines: diags.LineRange{
							First: 2,
							Last:  2,
						},
						Summary:  "third error",
						Details:  "third details",
						Reporter: "r2",
					},
				},
				{
					Path: discovery.Path{
						SymlinkTarget: "rule.yaml",
						Name:          "symlink.yaml",
					},
					ModifiedLines: []int{2, 3},
					Problem: checks.Problem{
						Severity: checks.Bug,
						Lines: diags.LineRange{
							First: 2,
							Last:  2,
						},
						Summary:  "fourth error",
						Details:  "fourth details",
						Reporter: "r2",
					},
				},
				{
					Path: discovery.Path{
						SymlinkTarget: "second.yaml",
						Name:          "second.yaml",
					},
					ModifiedLines: []int{1, 2, 3},
					Problem: checks.Problem{
						Anchor:   checks.AnchorBefore,
						Severity: checks.Bug,
						Lines: diags.LineRange{
							First: 2,
							Last:  2,
						},
						Summary:  "fifth error",
						Details:  "fifth details",
						Reporter: "r2",
					},
				},
				{
					Path: discovery.Path{
						SymlinkTarget: "second.yaml",
						Name:          "second.yaml",
					},
					ModifiedLines: []int{1, 2, 3},
					Problem: checks.Problem{
						Severity: checks.Bug,
						Lines: diags.LineRange{
							First: 2,
							Last:  2,
						},
						Summary:  "sixth error",
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
			maxComments: 50,
			summary: Summary{reports: []Report{
				{
					Path: discovery.Path{
						SymlinkTarget: "rule.yaml",
						Name:          "rule.yaml",
					},
					ModifiedLines: []int{2, 3},
					Problem: checks.Problem{
						Severity: checks.Bug,
						Lines: diags.LineRange{
							First: 2,
							Last:  2,
						},
						Summary:  "first error",
						Details:  "first details",
						Reporter: "r1",
					},
				},
				{
					Path: discovery.Path{
						SymlinkTarget: "rule.yaml",
						Name:          "rule.yaml",
					},
					ModifiedLines: []int{2, 3},
					Problem: checks.Problem{
						Severity: checks.Bug,
						Lines: diags.LineRange{
							First: 2,
							Last:  2,
						},
						Summary:  "second error",
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
		{
			description: "dedup identical reports",
			maxComments: 50,
			summary: Summary{reports: []Report{
				{
					Path: discovery.Path{
						SymlinkTarget: "rule.yaml",
						Name:          "rule.yaml",
					},
					ModifiedLines: []int{2, 3},
					Problem: checks.Problem{
						Severity: checks.Bug,
						Lines: diags.LineRange{
							First: 2,
							Last:  2,
						},
						Summary:  "my error",
						Details:  "my details",
						Reporter: "r1",
					},
				},
				{
					Path: discovery.Path{
						SymlinkTarget: "rule.yaml",
						Name:          "rule.yaml",
					},
					ModifiedLines: []int{2, 3},
					Problem: checks.Problem{
						Severity: checks.Bug,
						Lines: diags.LineRange{
							First: 2,
							Last:  2,
						},
						Summary:  "my error",
						Details:  "my details",
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
					Text:     commentBody("stop_sign", "Bug", "r1", "my error\n\nmy details"),
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
		{
			description: "dedup details",
			maxComments: 50,
			summary: Summary{reports: []Report{
				{
					Path: discovery.Path{
						SymlinkTarget: "rule.yaml",
						Name:          "rule.yaml",
					},
					ModifiedLines: []int{2, 3},
					Problem: checks.Problem{
						Severity: checks.Bug,
						Lines: diags.LineRange{
							First: 2,
							Last:  2,
						},
						Summary:  "first error",
						Details:  "shared details",
						Reporter: "r1",
					},
				},
				{
					Path: discovery.Path{
						SymlinkTarget: "rule.yaml",
						Name:          "rule.yaml",
					},
					ModifiedLines: []int{2, 3},
					Problem: checks.Problem{
						Severity: checks.Bug,
						Lines: diags.LineRange{
							First: 2,
							Last:  2,
						},
						Summary:  "second error",
						Details:  "shared details",
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
					Text:     commentBody("stop_sign", "Bug", "r1", "first error\n\n------\n\nsecond error\n\n------\n\nshared details"),
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
		{
			description: "maxComments 2",
			maxComments: 2,
			summary: Summary{reports: []Report{
				{
					Path: discovery.Path{
						SymlinkTarget: "rule.yaml",
						Name:          "rule.yaml",
					},
					ModifiedLines: []int{2, 3},
					Problem: checks.Problem{
						Severity: checks.Bug,
						Lines: diags.LineRange{
							First: 2,
							Last:  2,
						},
						Summary:  "first error",
						Details:  "first details",
						Reporter: "r1",
					},
				},
				{
					Path: discovery.Path{
						SymlinkTarget: "rule.yaml",
						Name:          "rule.yaml",
					},
					ModifiedLines: []int{2, 3},
					Problem: checks.Problem{
						Severity: checks.Warning,
						Lines: diags.LineRange{
							First: 3,
							Last:  3,
						},
						Summary:  "second error",
						Details:  "second details",
						Reporter: "r1",
					},
				},
				{
					Path: discovery.Path{
						SymlinkTarget: "rule.yaml",
						Name:          "rule.yaml",
					},
					ModifiedLines: []int{2, 3},
					Problem: checks.Problem{
						Severity: checks.Bug,
						Lines: diags.LineRange{
							First: 2,
							Last:  2,
						},
						Summary:  "third error",
						Details:  "third details",
						Reporter: "r2",
					},
				},
				{
					Path: discovery.Path{
						SymlinkTarget: "rule.yaml",
						Name:          "symlink.yaml",
					},
					ModifiedLines: []int{2, 3},
					Problem: checks.Problem{
						Severity: checks.Bug,
						Lines: diags.LineRange{
							First: 2,
							Last:  2,
						},
						Summary:  "fourth error",
						Details:  "fourth details",
						Reporter: "r2",
					},
				},
				{
					Path: discovery.Path{
						SymlinkTarget: "second.yaml",
						Name:          "second.yaml",
					},
					ModifiedLines: []int{1, 2, 3},
					Problem: checks.Problem{
						Anchor:   checks.AnchorBefore,
						Severity: checks.Bug,
						Lines: diags.LineRange{
							First: 2,
							Last:  2,
						},
						Summary:  "fifth error",
						Details:  "fifth details",
						Reporter: "r2",
					},
				},
				{
					Path: discovery.Path{
						SymlinkTarget: "second.yaml",
						Name:          "second.yaml",
					},
					ModifiedLines: []int{1, 2, 3},
					Problem: checks.Problem{
						Severity: checks.Bug,
						Lines: diags.LineRange{
							First: 2,
							Last:  2,
						},
						Summary:  "sixth error",
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
					Text:     "This pint run would create 5 comment(s), which is more than 2 limit configured for pint.\n3 comments were skipped and won't be visible on this PR.",
					Severity: "NORMAL",
					Anchor: BitBucketPendingCommentAnchor{
						DiffType: "EFFECTIVE",
					},
				},
			},
		},
		{
			description: "truncate long comments",
			maxComments: 2,
			summary: Summary{reports: []Report{
				{
					Path: discovery.Path{
						SymlinkTarget: "rule.yaml",
						Name:          "rule.yaml",
					},
					ModifiedLines: []int{2, 3},
					Problem: checks.Problem{
						Severity: checks.Bug,
						Lines: diags.LineRange{
							First: 2,
							Last:  2,
						},
						Summary:  strings.Repeat("X", maxCommentLength+1),
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
					Text:     ":stop_sign: **Bug** reported by [pint](https://cloudflare.github.io/pint/) **r1** check.\n\n------\n\n" + strings.Repeat("X", maxCommentLength-98-4) + " ...",
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
				tc.maxComments,
				tc.showDuplicates,
				nil,
			)
			comments := r.api.limitComments(r.api.makeComments(tc.summary, tc.changes))
			if diff := cmp.Diff(tc.comments, comments); diff != "" {
				t.Errorf("api.makeComments() returned wrong output (-want +got):\n%s", diff)
				return
			}
		})
	}
}
