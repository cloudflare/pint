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

// wrapText splits text at word boundaries so that each line fits within width.
func wrapText(text string, width int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	var lines []string
	current := words[0]
	curWidth := len(current)

	for _, word := range words[1:] {
		if curWidth+1+len(word) > width {
			lines = append(lines, current)
			current = word
			curWidth = len(word)
		} else {
			current += " " + word
			curWidth += 1 + len(word)
		}
	}
	lines = append(lines, current)
	return lines
}

// writeWrappedMessage writes msg to buf, wrapping it at word boundaries.
// indent is the number of leading spaces to write before each line.
// msgWidth is the maximum width of each line.
func writeWrappedMessage(buf *strings.Builder, msg string, color output.Color, indent, msgWidth int) {
	if len(msg) == 0 {
		return
	}

	if len(msg) <= msgWidth {
		buf.WriteString(strings.Repeat(" ", indent))
		buf.WriteString(output.MaybeColor(color, color == output.None, msg))
		buf.WriteRune('\n')
		return
	}

	for _, part := range wrapText(msg, msgWidth) {
		buf.WriteString(strings.Repeat(" ", indent))
		buf.WriteString(output.MaybeColor(color, color == output.None, part))
		buf.WriteRune('\n')
	}
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

// maxLineWidth is the maximum number of characters to print for a single line.
const maxLineWidth = 100

type lineTrim struct {
	line string // the (possibly trimmed) line text to display
}

// trimLine shortens lines longer than maxLineWidth while keeping all diagnostics visible.
// It uses AST-driven trimming for PromQL expressions, replacing sub-expressions that do
// not overlap with diagnostics with "...". If the line is not valid PromQL or no nodes
// can be replaced, the line is left untrimmed.
func trimLine(line string, diags []Diagnostic, diagPositions []PositionRanges, lineNum int) lineTrim {
	if len(line) <= maxLineWidth {
		return lineTrim{line: line}
	}

	// Try AST-based trimming. If it doesn't apply or the line is still
	// too long, keep the full line — we don't use blind window-based
	// trimming because it strips useful context.
	if newLine, ok := astTrimLine(line, diags, diagPositions, lineNum); ok {
		line = newLine
	}

	return lineTrim{line: line}
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

	// When two diagnostics have the exact same range, only the first one prints
	// underline characters. Subsequent ones skip the caret line but still print
	// their message aligned with the first diagnostic's underline.
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

		trim := trimLine(line, diags, diagPositions, lineIndex+1)
		line = trim.line

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
				caretLine := nextLine[i].String()

				indent := strings.IndexFunc(caretLine, func(r rune) bool { return r != ' ' })
				if indent < 0 {
					// For disabled-point diagnostics the caret line is all spaces,
					// so compute indent from the diagnostic's first column.
					indent = 0
					for _, pos := range diagPositions[i] {
						if pos.Line == lineIndex+1 && pos.FirstColumn > 0 {
							indent = digits + 3 + pos.FirstColumn - 1
							break
						}
					}
				}

				if !disablePoints[i] {
					buf.WriteString(output.MaybeColor(color, color == output.None, caretLine))
					buf.WriteRune('\n')
				}

				// If the first caret is past column 80, left-align the message
				// so it doesn't get pushed off-screen. Use the gutter width as
				// padding so the message lines up with the source content.
				caretCol := digits + 3 + indent
				if caretCol > 80 {
					indent = digits + 3
				}

				msgWidth := max(20, maxLineWidth+digits+3-indent)
				writeWrappedMessage(&buf, diags[i].Message, color, indent, msgWidth)
				nextLine[i].Reset()
			}
		}
	}

	return buf.String()
}
