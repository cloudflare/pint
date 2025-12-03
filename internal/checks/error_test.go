package checks_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/comments"
	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"
)

func TestErrorCheck(t *testing.T) {
	testCases := []struct {
		description string
		entry       *discovery.Entry
		problems    []checks.Problem
	}{
		{
			description: "rule error with default details",
			entry: &discovery.Entry{
				Rule: parser.Rule{
					Error: parser.ParseError{
						Err:  errors.New("some error"),
						Line: 1,
					},
				},
			},
			problems: []checks.Problem{
				{
					Reporter: "yaml/parse",
					Summary:  "This rule is not a valid Prometheus rule: `some error`.",
					Details: `This Prometheus rule is not valid.
This usually means that it's missing some required fields.`,
					Lines:    diags.LineRange{First: 1, Last: 1},
					Severity: checks.Fatal,
					Anchor:   checks.AnchorAfter,
				},
			},
		},
		{
			description: "rule error with custom details",
			entry: &discovery.Entry{
				Rule: parser.Rule{
					Error: parser.ParseError{
						Err:     errors.New("some error"),
						Details: "custom error details",
						Line:    1,
					},
				},
			},
			problems: []checks.Problem{
				{
					Reporter: "yaml/parse",
					Summary:  "This rule is not a valid Prometheus rule: `some error`.",
					Details:  "custom error details",
					Lines:    diags.LineRange{First: 1, Last: 1},
					Severity: checks.Fatal,
					Anchor:   checks.AnchorAfter,
				},
			},
		},
		{
			description: "comment error",
			entry: &discovery.Entry{
				PathError: comments.CommentError{
					Diagnostic: diags.Diagnostic{
						Message: "invalid comment",
						Pos:     diags.PositionRanges{{Line: 5}},
					},
				},
			},
			problems: []checks.Problem{
				{
					Reporter: "pint/comment",
					Summary:  "invalid comment",
					Details:  "",
					Lines:    diags.LineRange{First: 5, Last: 5},
					Severity: checks.Warning,
					Anchor:   checks.AnchorAfter,
					Diagnostics: []diags.Diagnostic{
						{
							Message: "invalid comment",
							Pos:     diags.PositionRanges{{Line: 5}},
						},
					},
				},
			},
		},
		{
			description: "owner error",
			entry: &discovery.Entry{
				PathError: comments.OwnerError{
					Diagnostic: diags.Diagnostic{
						Message: "invalid owner",
						Pos:     diags.PositionRanges{{Line: 3}},
					},
				},
			},
			problems: []checks.Problem{
				{
					Reporter: "rule/owner",
					Summary:  "invalid owner",
					Details:  "",
					Lines:    diags.LineRange{First: 3, Last: 3},
					Severity: checks.Bug,
					Anchor:   checks.AnchorAfter,
					Diagnostics: []diags.Diagnostic{
						{
							Message: "invalid owner",
							Pos:     diags.PositionRanges{{Line: 3}},
						},
					},
				},
			},
		},
		{
			description: "file ignore error",
			entry: &discovery.Entry{
				PathError: discovery.FileIgnoreError{
					Diagnostic: diags.Diagnostic{
						Message: "ignore error",
						Pos:     diags.PositionRanges{{Line: 2}},
					},
				},
			},
			problems: []checks.Problem{
				{
					Reporter: "ignore/file",
					Summary:  "ignore error",
					Details:  "",
					Lines:    diags.LineRange{First: 2, Last: 2},
					Severity: checks.Information,
					Anchor:   checks.AnchorAfter,
					Diagnostics: []diags.Diagnostic{
						{
							Message: "ignore error",
							Pos:     diags.PositionRanges{{Line: 2}},
						},
					},
				},
			},
		},
		{
			description: "parse error",
			entry: &discovery.Entry{
				PathError: parser.ParseError{
					Err:  errors.New("yaml syntax error"),
					Line: 10,
				},
			},
			problems: []checks.Problem{
				{
					Reporter: "yaml/parse",
					Summary:  "yaml syntax error",
					Details: `pint cannot read this file because YAML parser returned an error.
This usually means that you have an indention error or the file doesn't have the YAML structure required by Prometheus for [recording](https://prometheus.io/docs/prometheus/latest/configuration/recording_rules/) and [alerting](https://prometheus.io/docs/prometheus/latest/configuration/alerting_rules/) rules.
If this file is a template that will be rendered into valid YAML then you can instruct pint to ignore some lines using comments, see [pint docs](https://cloudflare.github.io/pint/ignoring.html).
`,
					Lines:    diags.LineRange{First: 10, Last: 10},
					Severity: checks.Fatal,
					Anchor:   checks.AnchorAfter,
				},
			},
		},
	}

	// This test doesn't use runTests() because ErrorCheck creates problems
	// with Diagnostics: nil for the default error case, while runTests() requires
	// all problems to have non-empty Diagnostics for snapshot testing.
	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			c := checks.NewErrorCheck(tc.entry)
			problems := c.Check(context.Background(), tc.entry, nil)
			require.Equal(t, tc.problems, problems)
		})
	}
}
