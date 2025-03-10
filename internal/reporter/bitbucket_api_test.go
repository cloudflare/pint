package reporter

import (
	"log/slog"
	"testing"

	"github.com/neilotoole/slogt"
	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/discovery"
)

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
