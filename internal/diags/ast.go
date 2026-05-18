package diags

import (
	"sort"
	"strings"

	promParser "github.com/prometheus/prometheus/promql/parser"
)

// intRange represents a half-open [start, end) range of 0-indexed positions.
type intRange struct {
	start int // 0-indexed, inclusive
	end   int // 0-indexed, exclusive
}

// parseASTRanges parses a PromQL expression and returns all AST node position ranges.
func parseASTRanges(expr string) []intRange {
	node, err := promParser.NewParser(promParser.Options{}).ParseExpr(expr)
	if err != nil {
		return nil
	}

	// Rough estimate: ~1 node per 4 bytes of expression.
	ranges := make([]intRange, 0, len(expr)/4)
	var walk func(n promParser.Node)
	walk = func(n promParser.Node) {
		pr := n.PositionRange()
		ranges = append(ranges, intRange{
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
// It returns the expression substring and its 0-indexed start column in the line.
// The caller must ensure isSingleLineExpr(diag, lineNum) is true.
func extractExprFromLine(diag Diagnostic, line string) (expr string, start int) {
	firstCol := diag.Pos[0].FirstColumn
	lastCol := diag.Pos[0].LastColumn
	for _, pos := range diag.Pos[1:] {
		firstCol = min(firstCol, pos.FirstColumn)
		lastCol = max(lastCol, pos.LastColumn)
	}
	start = firstCol - 1
	return line[start:lastCol], start
}

// offsetForCol computes the total column shift for a diagnostic position after
// applying a set of AST node replacements.
// reps is a list of [start, end) ranges within the expression that were
// replaced with "...". exprStart is the 0-indexed column where the expression
// begins in the original line. col is the diagnostic column to adjust.
func offsetForCol(reps []intRange, exprStart, col int) int {
	offset := 0
	for _, r := range reps {
		if exprStart+r.end < col {
			offset += 3 - (r.end - r.start)
		}
	}
	return offset
}

// collectDiagRanges returns [start, end) ranges within the expression that
// correspond to diagnostic positions on lineNum. exprStart is the 0-indexed
// column where the expression begins in the original line.
func collectDiagRanges(diagPositions []PositionRanges, lineNum, exprStart, exprLen int) []intRange {
	ranges := make([]intRange, 0, len(diagPositions))
	for _, dp := range diagPositions {
		for _, pos := range dp {
			if pos.Line != lineNum {
				continue
			}
			r0 := pos.FirstColumn - 1 - exprStart
			r1 := pos.LastColumn - exprStart
			if r0 >= 0 && r1 <= exprLen {
				ranges = append(ranges, intRange{start: r0, end: r1})
			}
		}
	}
	return ranges
}

// findReplacements returns a sorted, de-nested list of [start, end) ranges
// within the expression that can be replaced with "...". Only AST nodes that
// are at least 8 bytes wide and do not overlap any diagRange are eligible.
func findReplacements(astNodes, diagRanges []intRange) []intRange {
	reps := make([]intRange, 0, len(astNodes))
	for _, node := range astNodes {
		if node.end-node.start < 8 {
			continue
		}
		overlaps := false
		for _, dr := range diagRanges {
			if node.start < dr.end && node.end > dr.start {
				overlaps = true
				break
			}
		}
		if !overlaps {
			reps = append(reps, intRange{start: node.start, end: node.end})
		}
	}

	sort.Slice(reps, func(i, j int) bool {
		return reps[i].start < reps[j].start
	})

	// Remove nested replacements in-place (keep outermost).
	n := 0
	for _, r := range reps {
		nested := false
		for i := 0; i < n; i++ {
			if r.start >= reps[i].start && r.end <= reps[i].end {
				nested = true
				break
			}
		}
		if !nested {
			reps[n] = r
			n++
		}
	}
	return reps[:n]
}

// applyReplacements substitutes each [start, end) range in expr with "...".
func applyReplacements(expr string, reps []intRange) string {
	var buf strings.Builder
	last := 0
	for _, r := range reps {
		buf.WriteString(expr[last:r.start])
		buf.WriteString("...")
		last = r.end
	}
	buf.WriteString(expr[last:])
	return buf.String()
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

		expr, exprStart := extractExprFromLine(diag, line)
		if len(expr) < 10 {
			continue
		}

		astNodes := parseASTRanges(expr)
		if astNodes == nil {
			continue
		}

		diagRanges := collectDiagRanges(diagPositions, lineNum, exprStart, len(expr))
		reps := findReplacements(astNodes, diagRanges)
		if len(reps) == 0 {
			continue
		}

		trimmedExpr := applyReplacements(expr, reps)
		newLine := line[:exprStart] + trimmedExpr + line[exprStart+len(expr):]

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
