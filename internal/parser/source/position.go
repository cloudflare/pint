package source

import (
	"strings"

	"github.com/prometheus/prometheus/model/labels"
	promParser "github.com/prometheus/prometheus/promql/parser"
	"github.com/prometheus/prometheus/promql/parser/posrange"
)

const exprSpace = " \n\t"

func GetQueryFragment(expr string, pos posrange.PositionRange) string {
	return expr[pos.Start:pos.End]
}

func isOutside(pos posrange.PositionRange, outside []posrange.PositionRange) bool {
	for _, out := range outside {
		if pos.Start >= out.Start && pos.End <= out.End {
			return false
		}
	}
	return true
}

// FindFuncNamePosition returns the source position of a function name within
// the provided fragment.
//
// It matches fn case-insensitively, but only when the matched text is
// followed by optional whitespace and then `(`. This allows `by (...)` to
// match `by` while skipping the same text inside longer words such as
// `bytes`.
//
// The returned range covers only the function name. When converting it to a
// diagnostic, use:
//
// - FirstColumn = pos.Start + 1
// - LastColumn = pos.End.
func FindFuncNamePosition(expr string, within posrange.PositionRange, fn string) posrange.PositionRange {
	fragment := GetQueryFragment(expr, within)
	lower := strings.ToLower(fragment)
	fnLower := strings.ToLower(fn)
	offset := 0
	for {
		// Find next case-insensitive occurrence of fn.
		idx := strings.Index(lower[offset:], fnLower)
		if idx < 0 {
			return within
		}
		idx += offset
		end := idx + len(fn)
		// Check that fn is followed by optional whitespace then '('.
		// This skips false positives like "by" inside "bytes".
		for i := end; i < len(fragment); i++ {
			c := fragment[i]
			if c == '(' {
				return posrange.PositionRange{
					Start: within.Start + posrange.Pos(idx),
					End:   within.Start + posrange.Pos(end),
				}
			}
			if c != ' ' && c != '\n' && c != '\t' {
				break // Not whitespace, not '(' — this occurrence is inside a word.
			}
		}
		offset = idx + 1 // Try next occurrence.
	}
}

// FindFuncPosition returns the position of the whole `fn(...)` call inside the
// within fragment, from fn up to its closing ')'. When within spans more than
// one call, outside lists ranges the match must fall within, which lets callers
// pick the right `fn(...)` when the same keyword appears on both sides.
func FindFuncPosition(expr string, within posrange.PositionRange, fn string, outside []posrange.PositionRange) posrange.PositionRange {
	fragment := GetQueryFragment(expr, within)
	lower := strings.ToLower(fragment)
	fnLower := strings.ToLower(fn)
	for offset := 0; ; {
		idx := strings.Index(lower[offset:], fnLower)
		if idx < 0 {
			return within
		}
		idx += offset
		offset = idx + 1

		// fn must be followed by optional whitespace and then '(', otherwise
		// this is just fn appearing inside a longer word like "bytes".
		open := -1
		for i := idx + len(fn); i < len(fragment); i++ {
			c := fragment[i]
			if c == '(' {
				open = i
				break
			}
			if c != ' ' && c != '\n' && c != '\t' {
				break
			}
		}
		if open < 0 {
			continue
		}

		// The call ends at the first ')' after the opening paren.
		closeRel := strings.IndexByte(fragment[open:], ')')
		if closeRel < 0 {
			continue
		}

		pos := posrange.PositionRange{
			Start: within.Start + posrange.Pos(idx),
			End:   within.Start + posrange.Pos(open+closeRel+1),
		}
		if isOutside(pos, outside) {
			return pos
		}
		return within
	}
}

// findArgumentPosition looks for name inside the expr substring selected by
// within and returns the matching argument token position in the original expr.
//
// For example, if expr is `sum(foo) by(job, instance) > 0` and within selects
// `by(job, instance)`, then name="instance" returns the position of
// `instance` in the full expr.
//
// If expr is `foo * on(job) group_left(cluster) bar` and within selects
// `on(job)`, then name="job" returns the position of `job` in the full expr.
//
// If expr is `sum(foo) without(notify)` and within selects `without(notify)`,
// then name="notify" returns the position of `notify` in the full expr.
func findArgumentPosition(expr string, within posrange.PositionRange, name string) posrange.PositionRange {
	pos, ok := findUnquotedArgumentPosition(expr, within, name)
	if !ok {
		return within
	}
	return pos
}

// findBinOpsOperatorPosition returns the position of the op operator between the
// two sides of a binary expression, searching only the gap between them so a
// matching op inside either side is ignored.
func findBinOpsOperatorPosition(expr string, n *promParser.BinaryExpr, op string) posrange.PositionRange {
	within := posrange.PositionRange{
		Start: n.LHS.PositionRange().End + 1,
		End:   n.RHS.PositionRange().Start,
	}
	idx := strings.Index(GetQueryFragment(expr, within), op)
	if idx < 0 {
		return within
	}
	return posrange.PositionRange{
		Start: within.Start + posrange.Pos(idx),
		End:   within.Start + posrange.Pos(idx+len(op)),
	}
}

// findUnquotedMatcherName returns the position of the label name for the
// passed matcher. It returns a position only when the query writes the name
// without quotes. You can also write `foo.bar` as `{"foo.bar"="v"}`.
// That form does not need a feature flag, so this function skips quoted
// names and returns false.
func findUnquotedMatcherName(expr string, within posrange.PositionRange, m *labels.Matcher) (posrange.PositionRange, bool) {
	fragment := GetQueryFragment(expr, within)
	for off := 0; ; {
		idx := strings.Index(fragment[off:], m.Name)
		if idx < 0 {
			return posrange.PositionRange{}, false
		}
		idx += off
		off = idx + 1

		// The name must come before the match operator and an opening quote.
		// This test skips the same text inside label values.
		after, ok := strings.CutPrefix(strings.TrimLeft(fragment[idx+len(m.Name):], exprSpace), m.Type.String())
		if !ok || !strings.HasPrefix(strings.TrimLeft(after, exprSpace), `"`) {
			continue
		}
		if strings.HasSuffix(strings.TrimRight(fragment[:idx], exprSpace), `"`) {
			continue
		}
		return posrange.PositionRange{
			Start: within.Start + posrange.Pos(idx),
			End:   within.Start + posrange.Pos(idx+len(m.Name)),
		}, true
	}
}

// findUnquotedArgumentPosition looks for the last unquoted occurrence of
// name in the substring that within selects. The name must be surrounded by
// argument separators. A quoted name like `by ("service.name")` never
// matches, because a quote is not a separator. Such names do not need the
// promql-unquoted-utf8-names feature, so this function does not report them.
func findUnquotedArgumentPosition(expr string, within posrange.PositionRange, name string) (posrange.PositionRange, bool) {
	fragment := GetQueryFragment(expr, within)

	best := -1
	for off := 0; ; {
		i := strings.Index(fragment[off:], name)
		if i < 0 {
			break
		}
		i += off
		off = i + 1
		if isUnquotedArgument(fragment, i, len(name)) {
			best = i
		}
	}
	if best < 0 {
		return posrange.PositionRange{}, false
	}
	return posrange.PositionRange{
		Start: within.Start + posrange.Pos(best),
		End:   within.Start + posrange.Pos(best+len(name)),
	}, true
}

// isUnquotedArgument reports if the name that starts at the given position
// is written without quotes. A '(' or a ',' must come before the name, and
// a ')' or a ',' must come after the name. Whitespace around the name is
// permitted, any other character is not.
// For example `job` in `by ( job )` and in `by(job)` has no quotes. The
// same text inside `by ("job")` or `by (myjob)` does not match: a quote or
// a letter touches the name.
func isUnquotedArgument(fragment string, i, n int) bool {
	before := strings.TrimRight(fragment[:i], exprSpace)
	after := strings.TrimLeft(fragment[i+n:], exprSpace)
	return (strings.HasSuffix(before, "(") || strings.HasSuffix(before, ",")) &&
		(strings.HasPrefix(after, ")") || strings.HasPrefix(after, ","))
}

// FindMatcherPos locates 'name op "value"' (e.g. job=~"foo") in the expression
// fragment and returns the position spanning from "name" through the closing
// quote, with End exclusive (one past the closing quote) to match the other
// position functions and GetQueryFragment.
func FindMatcherPos(expr string, within posrange.PositionRange, m *labels.Matcher) posrange.PositionRange {
	fragment := GetQueryFragment(expr, within)
	// Build the full pattern: name + op + "value" (e.g. job=~"foo").
	pattern := m.Name + m.Type.String() + `"` + m.Value + `"`
	idx := strings.Index(fragment, pattern)
	if idx < 0 {
		return within
	}
	return posrange.PositionRange{
		Start: within.Start + posrange.Pos(idx),
		End:   within.Start + posrange.Pos(idx+len(pattern)),
	}
}
