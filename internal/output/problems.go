package output

import (
	"bufio"
	"cmp"
	"fmt"
	"slices"
	"strings"
)

func overlaps(n, minN, maxN int) bool {
	if n < minN {
		return false
	}
	if n > maxN {
		return false
	}
	return true
}

func countDigits(n int) (c int) {
	for n > 0 {
		n /= 10
		c++
	}
	return c
}

type Diagnostic struct {
	Message     string
	Line        int // 1-indexed
	FirstColumn int // 1-indexed
	LastColumn  int // 1-indexed
}

func InjectDiagnostics(content string, diags []Diagnostic, color Color, firstLine, lastLine int) string {
	slices.SortFunc(diags, func(a, b Diagnostic) int {
		return cmp.Or(
			cmp.Compare(b.FirstColumn, a.FirstColumn),
			cmp.Compare(a.LastColumn, b.LastColumn),
			cmp.Compare(a.Message, b.Message),
		)
	})

	var buf strings.Builder
	var lineIndex int
	nextLine := make([]strings.Builder, len(diags))
	needsNextLine := make([]bool, len(diags))
	offsets := make([]int, len(diags))

	disablePoints := make([]bool, len(diags))
	for i, a := range diags {
		for j := range i {
			b := diags[j]
			if a.Line == b.Line && a.FirstColumn == b.FirstColumn && a.LastColumn == b.LastColumn {
				disablePoints[i] = true
			}
		}
	}

	nrFmt := fmt.Sprintf("%%%dd", countDigits(lastLine))

	r := strings.NewReader(content)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		lineIndex++
		line := scanner.Text()
		doPrint := overlaps(lineIndex, firstLine, lastLine)

		for i, diag := range diags {
			if diag.Line == lineIndex {
				offsets[i] = 1
			}
			if offsets[i] > 0 && (overlaps(diag.FirstColumn, offsets[i], offsets[i]+len(line)) || overlaps(diag.LastColumn, offsets[i], offsets[i]+len(line))) {
				needsNextLine[i] = true
			} else {
				needsNextLine[i] = false
			}
		}

		if doPrint {
			prefix := fmt.Sprintf(nrFmt+" | ", lineIndex)
			buf.WriteString(MaybeColor(White, color == None, prefix))
			for i, ok := range needsNextLine {
				if ok {
					nextLine[i].WriteString(strings.Repeat(" ", len(prefix)))
				}
			}
		}

		for _, c := range line {
			if doPrint {
				buf.WriteRune(c)
			}

			for i, ok := range needsNextLine {
				if ok {
					if offsets[i] < diags[i].FirstColumn {
						nextLine[i].WriteRune(' ')
					} else if offsets[i] >= diags[i].FirstColumn && offsets[i] <= diags[i].LastColumn {
						if disablePoints[i] {
							nextLine[i].WriteRune(' ')
						} else {
							nextLine[i].WriteRune('^')
						}
					}
				}
			}

			for i, v := range offsets {
				if v > 0 {
					offsets[i]++
				}
			}
		}

		if doPrint {
			buf.WriteRune('\n')
		}

		for i, v := range offsets {
			if v > 0 {
				offsets[i]++
			}
		}

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
