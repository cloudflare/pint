package diags

import (
	"sort"
	"strings"

	promParser "github.com/prometheus/prometheus/promql/parser"
)

// astNodeRange represents a range in the expression that corresponds to an AST node.
type astNodeRange struct {
	start int // 0-indexed, inclusive
	end   int // 0-indexed, exclusive
}

// parseASTRanges parses a PromQL expression and returns all AST node position ranges.
func parseASTRanges(expr string) []astNodeRange {
	node, err := promParser.NewParser(promParser.Options{}).ParseExpr(expr)
	if err != nil {
		return nil
	}

	// Rough estimate: ~1 node per 4 bytes of expression.
	ranges := make([]astNodeRange, 0, len(expr)/4)
	var walk func(n promParser.Node)
	walk = func(n promParser.Node) {
		pr := n.PositionRange()
		ranges = append(ranges, astNodeRange{
			start: int(pr.Start),
			end:   int(pr.End),
		})
		for _, child := range promParser.Children(n) {
			walk(child)
		}
	}
	walk(node)
	return ranges
}

// isSingleLineExpr reports whether all positions of diag are on lineNum.
func isSingleLineExpr(diag Diagnostic, lineNum int) bool {
	for _, pos := range diag.Pos {
		if pos.Line != lineNum {
			return false
		}
	}
	return len(diag.Pos) > 0
}

// extractExprFromLine pulls the PromQL expression text from line using diag.Pos.
// It returns the expression substring, its 0-indexed start column in the line,
// and true on success.
func extractExprFromLine(diag Diagnostic, line string, lineNum int) (expr string, start int, ok bool) {
	var firstCol, lastCol int
	var found bool
	for _, pos := range diag.Pos {
		if pos.Line != lineNum {
			continue
		}
		if !found {
			firstCol = pos.FirstColumn
			lastCol = pos.LastColumn
			found = true
			continue
		}
		firstCol = min(firstCol, pos.FirstColumn)
		lastCol = max(lastCol, pos.LastColumn)
	}
	if !found {
		return "", 0, false
	}

	start = firstCol - 1
	end := lastCol
	if start < 0 || end > len(line) {
		return "", 0, false
	}
	return line[start:end], start, true
}

// offsetForCol computes the total column shift for a diagnostic position after
// applying a set of AST node replacements.
// reps is a list of [start, end) ranges within the expression that were
// replaced with "...". exprStart is the 0-indexed column where the expression
// begins in the original line. col is the diagnostic column to adjust.
func offsetForCol(reps [][2]int, exprStart, col int) int {
	offset := 0
	for _, r := range reps {
		if exprStart+r[1] < col {
			offset += 3 - (r[1] - r[0])
		}
	}
	return offset
}

// astTrimLine parses PromQL expressions on the given line and replaces AST nodes
// that don't overlap with any diagnostic. It updates diagPositions in place to
// account for length changes. It returns the modified line and true if any
// replacement was made.
func astTrimLine(line string, diags []Diagnostic, diagPositions []PositionRanges, lineNum int) (string, bool) {
	for _, diag := range diags {
		if !isSingleLineExpr(diag, lineNum) {
			continue
		}

		expr, exprStart, ok := extractExprFromLine(diag, line, lineNum)
		if !ok || len(expr) < 10 {
			continue
		}

		astNodes := parseASTRanges(expr)
		if astNodes == nil {
			continue
		}

		diagRanges := make([][2]int, 0, len(diagPositions))
		for _, dp := range diagPositions {
			for _, pos := range dp {
				if pos.Line != lineNum {
					continue
				}
				r0 := pos.FirstColumn - 1 - exprStart
				r1 := pos.LastColumn - exprStart
				if r0 >= 0 && r1 <= len(expr) {
					diagRanges = append(diagRanges, [2]int{r0, r1})
				}
			}
		}

		reps := make([][2]int, 0, len(astNodes))
		for _, node := range astNodes {
			if node.end-node.start < 8 {
				continue
			}
			overlaps := false
			for _, dr := range diagRanges {
				if node.start < dr[1] && node.end > dr[0] {
					overlaps = true
					break
				}
			}
			if !overlaps {
				reps = append(reps, [2]int{node.start, node.end})
			}
		}

		if len(reps) == 0 {
			continue
		}

		sort.Slice(reps, func(i, j int) bool {
			return reps[i][0] < reps[j][0]
		})

		// Remove nested replacements in-place (keep outermost).
		n := 0
		for _, r := range reps {
			nested := false
			for i := 0; i < n; i++ {
				if r[0] >= reps[i][0] && r[1] <= reps[i][1] {
					nested = true
					break
				}
			}
			if !nested {
				reps[n] = r
				n++
			}
		}
		reps = reps[:n]

		// Build trimmed expression.
		var buf strings.Builder
		last := 0
		for _, r := range reps {
			buf.WriteString(expr[last:r[0]])
			buf.WriteString("...")
			last = r[1]
		}
		buf.WriteString(expr[last:])
		trimmedExpr := buf.String()

		// Rebuild the line.
		newLine := line[:exprStart] + trimmedExpr + line[exprStart+len(expr):]

		// Adjust diagPositions to account for length changes.
		for i := range diagPositions {
			for j := range diagPositions[i] {
				if diagPositions[i][j].Line != lineNum {
					continue
				}
				diagPositions[i][j].FirstColumn += offsetForCol(reps, exprStart, diagPositions[i][j].FirstColumn)
				diagPositions[i][j].LastColumn += offsetForCol(reps, exprStart, diagPositions[i][j].LastColumn)
			}
		}

		return newLine, true
	}

	return line, false
}
