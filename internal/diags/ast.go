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

// nodeRangesFromAST walks a parsed PromQL AST and maps each node's position
// range to line/column using pos. Only nodes that fit on a single line are
// included.
func nodeRangesFromAST(node promParser.Node, pos PositionRanges) []PositionRange {
	posLen := pos.Len()
	var ranges []PositionRange
	var walk func(n promParser.Node)
	walk = func(n promParser.Node) {
		pr := n.PositionRange()
		start := int(pr.Start)
		end := int(pr.End)
		if start >= 0 && end > start && end <= posLen {
			startLine, startCol, ok1 := posAtOffset(pos, start)
			endLine, endCol, ok2 := posAtOffset(pos, end-1)
			if ok1 && ok2 && startLine == endLine {
				ranges = append(ranges, PositionRange{
					Line:        startLine,
					FirstColumn: startCol,
					LastColumn:  endCol,
				})
			}
		}
		for _, child := range promParser.Children(n) {
			walk(child)
		}
	}
	walk(node)
	return ranges
}

// posAtOffset returns the line and column for a 0-indexed byte offset
// within the flat expression text mapped by pos.
func posAtOffset(pos PositionRanges, offset int) (line, col int, ok bool) {
	var n int
	for _, pr := range pos {
		span := pr.LastColumn - pr.FirstColumn + 1
		if offset < n+span {
			line = pr.Line
			col = pr.FirstColumn + (offset - n)
			ok = true
			break
		}
		n += span
	}
	return line, col, ok
}

// offsetForCol computes the total column shift for a diagnostic position after
// applying a set of AST node replacements.
// reps is a list of [start, end) ranges within the expression that were
// replaced with "...". exprStart is the 0-indexed column where the expression
// begins in the original line. col is the diagnostic column to adjust.
func offsetForCol(reps []intRange, exprStart, col int) int {
	var offset int
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
	var n int
	for _, r := range reps {
		nested := false
		for i := range n {
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
	var last int
	for _, r := range reps {
		buf.WriteString(expr[last:r.start])
		buf.WriteString("...")
		last = r.end
	}
	buf.WriteString(expr[last:])
	return buf.String()
}

// exprNodesOnLine walks diag.Expr and returns AST node ranges on lineNum
// as intRange relative to exprStart.
func exprNodesOnLine(diag Diagnostic, lineNum, exprStart int) []intRange {
	allNodes := nodeRangesFromAST(diag.Expr, diag.Pos)
	var out []intRange
	for _, nr := range allNodes {
		if nr.Line != lineNum {
			continue
		}
		out = append(out, intRange{
			start: nr.FirstColumn - 1 - exprStart,
			end:   nr.LastColumn - exprStart,
		})
	}
	return out
}

// astTrimLine replaces AST nodes that don't overlap with any diagnostic on the
// given line using the pre-parsed AST stored in diag.Expr. Diagnostics without
// Expr are skipped. It updates diagPositions in place to account for length
// changes and returns the modified line and true if any replacement was made.
func astTrimLine(line string, diags []Diagnostic, diagPositions []PositionRanges, lineNum int) (string, bool) {
	for _, diag := range diags {
		var lineNodes []intRange
		var exprStart int

		if diag.Expr == nil {
			continue
		}
		exprStart = len(line) - len(strings.TrimLeft(line, " \t"))
		lineNodes = exprNodesOnLine(diag, lineNum, exprStart)

		diagRanges := collectDiagRanges(diagPositions, lineNum, exprStart, len(line)-exprStart)
		nodes := lineNodes
		if len(diagRanges) == 0 {
			if len(nodes) > 0 {
				nodes = nodes[1:]
			}
		}

		reps := findReplacements(nodes, diagRanges)
		if len(reps) == 0 {
			continue
		}

		trimmedExpr := applyReplacements(line[exprStart:], reps)
		newLine := line[:exprStart] + trimmedExpr

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
