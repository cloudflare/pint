package source

import (
	"strings"

	"github.com/prometheus/prometheus/model/labels"
	promParser "github.com/prometheus/prometheus/promql/parser"
	"github.com/prometheus/prometheus/promql/parser/posrange"
)

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
	fragment := GetQueryFragment(expr, within)

	isSpace := func(b byte) bool {
		return b == ' ' || b == '\n' || b == '\t'
	}
	// Match the right-most name that sits as a complete argument, i.e. one
	// bordered by '(' or ',' on the left and ',' or ')' on the right.
	best := -1
	for off := 0; ; {
		i := strings.Index(fragment[off:], name)
		if i < 0 {
			break
		}
		i += off
		off = i + 1

		// The byte opening the argument must be '(' or a ',' separator.
		p := i - 1
		for p >= 0 && isSpace(fragment[p]) {
			p--
		}
		if p < 0 || (fragment[p] != '(' && fragment[p] != ',') {
			continue
		}

		// The byte closing the argument must be ')' or a ',' separator.
		q := i + len(name)
		for q < len(fragment) && isSpace(fragment[q]) {
			q++
		}
		if q >= len(fragment) || (fragment[q] != ')' && fragment[q] != ',') {
			continue
		}

		best = i
	}
	if best < 0 {
		return within
	}
	return posrange.PositionRange{
		Start: within.Start + posrange.Pos(best),
		End:   within.Start + posrange.Pos(best+len(name)),
	}
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
