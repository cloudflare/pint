package output

import (
	"fmt"
	"strings"
)

func countDigits(n int) (c int) {
	for n > 0 {
		n /= 10
		c++
	}
	return c
}

type Diagnostic struct {
	Message     string
	Pos         PositionRanges
	FirstColumn int // 1-indexed
	LastColumn  int // 1-indexed
}

func InjectDiagnostics(content string, diags []Diagnostic, color Color, firstLine, lastLine int) string {
	diagPositions := make([]PositionRanges, len(diags))
	for i, diag := range diags {
		diagPositions[i] = readRange(diag.FirstColumn, diag.LastColumn, diag.Pos)
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

	nrFmt := fmt.Sprintf("%%%dd", countDigits(lastLine))

	for lineIndex, line := range strings.Split(content, "\n") {

		if lineIndex+1 < firstLine {
			continue
		}
		if lineIndex+1 > lastLine {
			break
		}

		for i := range diags {
			needsNextLine[i] = false
			for _, pos := range diagPositions[i] {
				if pos.Line == lineIndex+1 {
					needsNextLine[i] = true
				}
			}
		}

		prefix := fmt.Sprintf(nrFmt+" | ", lineIndex+1)
		buf.WriteString(MaybeColor(White, color == None, prefix))
		for i, ok := range needsNextLine {
			if ok {
				nextLine[i].WriteString(strings.Repeat(" ", len(prefix)))
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
				buf.WriteString(MaybeColor(color, color == None, nextLine[i].String()))
				buf.WriteRune(' ')
				buf.WriteString(MaybeColor(color, color == None, diags[i].Message))
				buf.WriteRune('\n')
				nextLine[i].Reset()
			}
		}
	}

	return buf.String()
}
