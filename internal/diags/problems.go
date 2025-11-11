package diags

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/cloudflare/pint/internal/output"
)

func countDigits(n int) (c int) {
	for n > 0 {
		n /= 10
		c++
	}
	return c
}

type Kind uint8

const (
	Issue Kind = iota
	Context
)

type Diagnostic struct {
	Message     string
	Pos         PositionRanges `yaml:"-"`
	FirstColumn int            // 1-indexed
	LastColumn  int            // 1-indexed
	Kind        Kind
}

func lineCoverage(diags []Diagnostic) (lines []int) {
	for _, diag := range diags {
		for _, pos := range diag.Pos {
			if !slices.Contains(lines, pos.Line) {
				lines = append(lines, pos.Line)
			}
		}
	}
	slices.Sort(lines)
	return lines
}

func InjectDiagnostics(content string, d []Diagnostic, color output.Color) string {
	diags := make([]Diagnostic, len(d))
	copy(diags, d)

	lines := lineCoverage(diags)
	lastLine := slices.Max(lines)

	slices.SortStableFunc(diags, func(a, b Diagnostic) int {
		return cmp.Compare(b.FirstColumn, a.FirstColumn)
	})

	diagPositions := make([]PositionRanges, len(diags))
	for i, diag := range diags {
		dl := diag.Pos.Len()
		diagPositions[i] = readRange(
			min(diag.FirstColumn, dl),
			min(diag.LastColumn, dl),
			diag.Pos,
		)
	}

	var buf strings.Builder
	nextLine := make([]strings.Builder, len(diags))
	needsNextLine := make([]bool, len(diags))

	disablePoints := make([]bool, len(diags))
	for i, a := range diags {
		for j := range i {
			b := diags[j]
			if a.FirstColumn == b.FirstColumn && a.LastColumn == b.LastColumn {
				disablePoints[i] = true
			}
		}
	}

	digits := countDigits(lastLine)
	nrFmt := fmt.Sprintf("%%%dd", digits)

	var lastWriteLine int
	for lineIndex, line := range strings.Split(content, "\n") {

		if lineIndex+1 > lastLine {
			break
		}
		if !slices.Contains(lines, lineIndex+1) {
			continue
		}

		for i := range diags {
			needsNextLine[i] = false
			if lineIndex+1 == diagPositions[i].Lines().Last {
				needsNextLine[i] = true
			}
		}

		prefix := fmt.Sprintf(nrFmt+" | ", lineIndex+1)

		if lastWriteLine > 0 && lineIndex+1-lastWriteLine > 1 {
			buf.WriteString(output.MaybeColor(output.White, color == output.None, strings.Repeat(" ", digits)))
			buf.WriteString(" | [...]\n")
		}
		lastWriteLine = lineIndex + 1

		buf.WriteString(output.MaybeColor(output.White, color == output.None, prefix))
		for i, ok := range needsNextLine {
			if ok {
				nextLine[i].WriteString(strings.Repeat(" ", digits+3))
			}
		}

		for columnIndex, r := range line {
			buf.WriteRune(r)

			for i, ok := range needsNextLine {
				if !ok {
					continue
				}
				for _, pos := range diagPositions[i] {
					if pos.Line != lineIndex+1 {
						continue
					}
					before := pos.FirstColumn > columnIndex+1
					inside := pos.FirstColumn <= columnIndex+1 && pos.LastColumn >= columnIndex+1
					switch {
					case before:
						nextLine[i].WriteRune(' ')
					case inside && disablePoints[i]:
						nextLine[i].WriteRune(' ')
					case inside && !disablePoints[i]:
						nextLine[i].WriteRune('^')
					}
				}
			}
		}
		buf.WriteRune('\n')

		for i, ok := range needsNextLine {
			if ok {
				buf.WriteString(output.MaybeColor(color, color == output.None, nextLine[i].String()))
				buf.WriteRune(' ')
				buf.WriteString(output.MaybeColor(color, color == output.None, diags[i].Message))
				buf.WriteRune('\n')
				nextLine[i].Reset()
			}
		}
	}

	return buf.String()
}
