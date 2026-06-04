package source_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/parser/source"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/promql/parser/posrange"
)

// highlightRange returns expr with a caret line under each line, marking the
// bytes selected by pos with '^'. Carets stay aligned with the expr above them
// (no shifting); tabs and newlines are copied through so indentation lines up.
func highlightRange(expr string, pos posrange.PositionRange) string {
	start := min(int(pos.Start), len(expr))
	end := min(int(pos.End), len(expr))
	if start > end {
		start = end
	}

	marker := []byte(expr)
	for i := range marker {
		switch {
		case marker[i] == '\n' || marker[i] == '\t':
		case i >= start && i < end:
			marker[i] = '^'
		default:
			marker[i] = ' '
		}
	}

	exprLines := strings.Split(expr, "\n")
	markerLines := strings.Split(string(marker), "\n")
	var out []string
	for i := range exprLines {
		out = append(out, exprLines[i])
		if strings.ContainsRune(markerLines[i], '^') {
			out = append(out, markerLines[i])
		}
	}
	return strings.Join(out, "\n")
}

func matchPositionSnapshot(t *testing.T, content string) {
	t.Helper()
	snaps.WithConfig(snaps.Dir("."), snaps.Filename("position_test")).MatchSnapshot(t, content)
}

func renderOutside(outside []posrange.PositionRange) string {
	if len(outside) == 0 {
		return "none"
	}
	parts := make([]string, 0, len(outside))
	for _, o := range outside {
		parts = append(parts, fmt.Sprintf("%d-%d", o.Start, o.End))
	}
	return strings.Join(parts, ", ")
}

func TestLabelsSourceFindArgumentPositionCoverage(t *testing.T) {
	type testCase struct {
		description string
		expr        string
		label       string
	}

	testCases := []testCase{
		{
			description: "skip label name embedded in another identifier",
			expr:        `sum(foo) by(job, notjob)`,
			label:       "job",
		},
		{
			description: "skip whitespace before closing delimiter",
			expr:        `sum(foo) by(job )`,
			label:       "job",
		},
		{
			description: "skip whitespace before label name",
			expr:        `sum(foo) by( job)`,
			label:       "job",
		},
		{
			description: "skip multiple spaces around label name",
			expr:        `sum(foo) by(   job   )`,
			label:       "job",
		},
		{
			description: "skip tabs around label name",
			expr:        "sum(foo) by(\tjob\t)",
			label:       "job",
		},
		{
			description: "skip newlines around label name",
			expr:        "sum(foo) by(\njob\n)",
			label:       "job",
		},
		{
			description: "skip mixed whitespace in multi-argument list",
			expr:        "sum(foo) by(\n  job,\n  instance\n)",
			label:       "job",
		},
		{
			description: "skip label name that is a prefix of an earlier argument",
			expr:        `sum(foo) by(jobx, job)`,
			label:       "job",
		},
		{
			description: "skip invalid suffix and keep searching for earlier valid match",
			expr:        `(foo * on(job) group_left(cluster) bar) and on(job) baz{job="x"}`,
			label:       "job",
		},
		{
			description: "find middle label in multiline grouping with tabs and spaces",
			expr:        "sum(foo) by(\n\tjob,\n\t   instance,\n        region\n)",
			label:       "instance",
		},
		{
			description: "find last label in multiline grouping with tabs and spaces",
			expr:        "sum(foo) by(\n\tjob,\n\t   instance,\n        region\n)",
			label:       "region",
		},
		{
			description: "find first label in multiline grouping with tabs and spaces",
			expr:        "sum(foo) by(\n\tjob,\n\t   instance,\n        region\n)",
			label:       "job",
		},
		{
			description: "label named like the aggregation it groups",
			expr:        `sum(sum) by(sum)`,
			label:       "sum",
		},
		{
			description: "label named like the metric inside without grouping",
			expr:        `sum(rate) without(rate)`,
			label:       "rate",
		},
		{
			description: "find label in deeply nested aggregation grouping",
			expr:        `sum(sum(sum(foo) by(job)) by(job)) by(job)`,
			label:       "job",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			n, err := parser.DecodeExpr(tc.expr)
			require.NoError(t, err)

			output := source.LabelsSource(tc.expr, n.Expr)
			require.NotEmpty(t, output)

			var fragment posrange.PositionRange
			for _, src := range output {
				src.WalkSources(func(s *source.Source, _ *source.Join, _ *source.Unless) {
					label, ok := s.Labels[tc.label]
					if !ok {
						return
					}
					if fragment == (posrange.PositionRange{}) {
						fragment = label.Fragment
						return
					}
					if label.Fragment.End-label.Fragment.Start < fragment.End-fragment.Start {
						fragment = label.Fragment
					}
				})
			}

			require.NotEqual(t, posrange.PositionRange{}, fragment)
			matchPositionSnapshot(t, fmt.Sprintf(
				"label: %s\nexpr:\n%s",
				tc.label,
				highlightRange(tc.expr, fragment),
			))
		})
	}
}

func TestGetQueryFragment(t *testing.T) {
	type testCase struct {
		description string
		expr        string
		expected    string
		pos         posrange.PositionRange
	}

	testCases := []testCase{
		{
			description: "extracts the leading token",
			expr:        "sum(foo)",
			pos:         posrange.PositionRange{Start: 0, End: 3},
			expected:    "sum",
		},
		{
			description: "extracts a token in the middle",
			expr:        "sum(foo)",
			pos:         posrange.PositionRange{Start: 4, End: 7},
			expected:    "foo",
		},
		{
			description: "returns empty string for an empty range",
			expr:        "sum(foo)",
			pos:         posrange.PositionRange{Start: 3, End: 3},
			expected:    "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			require.Equal(t, tc.expected, source.GetQueryFragment(tc.expr, tc.pos))
		})
	}
}

func TestFindFuncNamePosition(t *testing.T) {
	type testCase struct {
		description string
		expr        string
		fn          string
		within      posrange.PositionRange
	}

	testCases := []testCase{
		{
			description: "returns within when fn is absent",
			expr:        "sum(foo)",
			within:      posrange.PositionRange{Start: 0, End: 8},
			fn:          "rate",
		},
		{
			description: "matches the function name before the paren",
			expr:        "sum(foo)",
			within:      posrange.PositionRange{Start: 0, End: 8},
			fn:          "sum",
		},
		{
			description: "matches with whitespace before the paren",
			expr:        "sum (foo)",
			within:      posrange.PositionRange{Start: 0, End: 9},
			fn:          "sum",
		},
		{
			description: "matches case-insensitively",
			expr:        "SUM(foo)",
			within:      posrange.PositionRange{Start: 0, End: 8},
			fn:          "sum",
		},
		{
			description: "skips an occurrence inside a word and matches the call",
			expr:        "sumx + sum(foo)",
			within:      posrange.PositionRange{Start: 0, End: 15},
			fn:          "sum",
		},
		{
			description: "respects the within offset",
			expr:        "xx sum(foo)",
			within:      posrange.PositionRange{Start: 3, End: 11},
			fn:          "sum",
		},
		{
			description: "returns the first occurrence when the name repeats",
			expr:        "sum(sum) by(sum)",
			within:      posrange.PositionRange{Start: 0, End: 16},
			fn:          "sum",
		},
		{
			description: "matches the call and not a longer identifier with the same prefix",
			expr:        "sum(summary)",
			within:      posrange.PositionRange{Start: 0, End: 12},
			fn:          "sum",
		},
		{
			description: "skips the name inside a leading word and matches the later call",
			expr:        "summary + sum(x)",
			within:      posrange.PositionRange{Start: 0, End: 16},
			fn:          "sum",
		},
		{
			description: "matches across newline and tab before the paren",
			expr:        "SuM\n\t(x)",
			within:      posrange.PositionRange{Start: 0, End: 8},
			fn:          "sum",
		},
		{
			description: "returns within when the name only appears inside a word",
			expr:        "bytes",
			within:      posrange.PositionRange{Start: 0, End: 5},
			fn:          "by",
		},
		{
			description: "returns within when the name is followed by a non-paren token",
			expr:        "by job",
			within:      posrange.PositionRange{Start: 0, End: 6},
			fn:          "by",
		},
		{
			description: "matches an aggregation wrapping a nested call",
			expr:        "sum(rate(foo[5m]))",
			within:      posrange.PositionRange{Start: 0, End: 18},
			fn:          "sum",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			got := source.FindFuncNamePosition(tc.expr, tc.within, tc.fn)
			matchPositionSnapshot(t, fmt.Sprintf(
				"fn: %s\nexpr:\n%s",
				tc.fn,
				highlightRange(tc.expr, got),
			))
		})
	}
}

func TestFindFuncPosition(t *testing.T) {
	type testCase struct {
		description string
		expr        string
		fn          string
		outside     []posrange.PositionRange
		within      posrange.PositionRange
	}

	testCases := []testCase{
		{
			description: "returns within when fn is absent",
			expr:        "x sum(foo)",
			within:      posrange.PositionRange{Start: 0, End: 10},
			fn:          "rate",
			outside:     nil,
		},
		{
			description: "matches the whole call when no outside ranges are given",
			expr:        "x sum(foo)",
			within:      posrange.PositionRange{Start: 0, End: 10},
			fn:          "sum",
			outside:     nil,
		},
		{
			description: "returns within when every match is contained in an outside range",
			expr:        "x sum(foo)",
			within:      posrange.PositionRange{Start: 0, End: 10},
			fn:          "sum",
			outside:     []posrange.PositionRange{{Start: 0, End: 100}},
		},
		{
			description: "matches when the outside range does not contain the match",
			expr:        "x sum(foo)",
			within:      posrange.PositionRange{Start: 0, End: 10},
			fn:          "sum",
			outside:     []posrange.PositionRange{{Start: 50, End: 60}},
		},
		{
			description: "skips fn not followed by a paren and matches a later call",
			expr:        "sumx + sum(foo)",
			within:      posrange.PositionRange{Start: 0, End: 15},
			fn:          "sum",
			outside:     nil,
		},
		{
			description: "returns within when the call has no closing paren",
			expr:        "sum(foo",
			within:      posrange.PositionRange{Start: 0, End: 7},
			fn:          "sum",
			outside:     nil,
		},
		{
			description: "matches the first call when the name repeats",
			expr:        "sum(sum) by(sum)",
			within:      posrange.PositionRange{Start: 0, End: 16},
			fn:          "sum",
			outside:     nil,
		},
		{
			description: "matches a grouping keyword call",
			expr:        "sum(sum) by(sum)",
			within:      posrange.PositionRange{Start: 0, End: 16},
			fn:          "by",
			outside:     nil,
		},
		{
			description: "matches a multiline grouping with mixed whitespace",
			expr:        "sum(foo)\nby(\n\tjob\n)",
			within:      posrange.PositionRange{Start: 0, End: 19},
			fn:          "by",
			outside:     nil,
		},
		{
			description: "ends a nested call at the first closing paren",
			expr:        "scalar(vector(1))",
			within:      posrange.PositionRange{Start: 0, End: 17},
			fn:          "scalar",
			outside:     nil,
		},
		{
			description: "finds the keyword between operands when the operands are excluded",
			expr:        "aa * on(b) cc",
			within:      posrange.PositionRange{Start: 0, End: 13},
			fn:          "on",
			outside:     []posrange.PositionRange{{Start: 0, End: 2}, {Start: 11, End: 13}},
		},
		{
			description: "returns within when the only match is contained in an outside range",
			expr:        "x on(b)",
			within:      posrange.PositionRange{Start: 0, End: 7},
			fn:          "on",
			outside:     []posrange.PositionRange{{Start: 0, End: 100}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			got := source.FindFuncPosition(tc.expr, tc.within, tc.fn, tc.outside)
			matchPositionSnapshot(t, fmt.Sprintf(
				"fn: %s\noutside: %s\nexpr:\n%s",
				tc.fn,
				renderOutside(tc.outside),
				highlightRange(tc.expr, got),
			))
		})
	}
}

func TestFindMatcherPos(t *testing.T) {
	type testCase struct {
		matcher     *labels.Matcher
		description string
		expr        string
		within      posrange.PositionRange
	}

	testCases := []testCase{
		{
			description: "matches name op and quoted value",
			expr:        `foo{job=~"bar"}`,
			within:      posrange.PositionRange{Start: 0, End: 15},
			matcher:     labels.MustNewMatcher(labels.MatchRegexp, "job", "bar"),
		},
		{
			description: "returns within when the matcher is absent",
			expr:        `foo{job=~"bar"}`,
			within:      posrange.PositionRange{Start: 0, End: 15},
			matcher:     labels.MustNewMatcher(labels.MatchRegexp, "job", "baz"),
		},
		{
			description: "matches an equality matcher",
			expr:        `foo{job="bar"}`,
			within:      posrange.PositionRange{Start: 0, End: 14},
			matcher:     labels.MustNewMatcher(labels.MatchEqual, "job", "bar"),
		},
		{
			description: "matches a negative equality matcher",
			expr:        `foo{job!="bar"}`,
			within:      posrange.PositionRange{Start: 0, End: 15},
			matcher:     labels.MustNewMatcher(labels.MatchNotEqual, "job", "bar"),
		},
		{
			description: "matches a negative regexp matcher",
			expr:        `foo{job!~"bar"}`,
			within:      posrange.PositionRange{Start: 0, End: 15},
			matcher:     labels.MustNewMatcher(labels.MatchNotRegexp, "job", "bar"),
		},
		{
			description: "matches the requested matcher among several",
			expr:        `foo{a="1",job=~"bar"}`,
			within:      posrange.PositionRange{Start: 0, End: 21},
			matcher:     labels.MustNewMatcher(labels.MatchRegexp, "job", "bar"),
		},
		{
			description: "returns within when the value contains an escaped quote",
			expr:        `foo{job="a\"b"}`,
			within:      posrange.PositionRange{Start: 0, End: 15},
			matcher:     labels.MustNewMatcher(labels.MatchEqual, "job", `a"b`),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			got := source.FindMatcherPos(tc.expr, tc.within, tc.matcher)
			matchPositionSnapshot(t, fmt.Sprintf(
				"matcher: %s\nexpr:\n%s",
				tc.matcher.String(),
				highlightRange(tc.expr, got),
			))
		})
	}
}
