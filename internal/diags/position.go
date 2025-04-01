package diags

import (
	"fmt"
	"strconv"

	"gopkg.in/yaml.v3"
)

type LineRange struct {
	First int
	Last  int
}

func (lr LineRange) String() string {
	if lr.First == lr.Last {
		return strconv.Itoa(lr.First)
	}
	return fmt.Sprintf("%d-%d", lr.First, lr.Last)
}

func (lr LineRange) Expand() []int {
	lines := make([]int, 0, lr.Last-lr.First+1)
	for i := lr.First; i <= lr.Last; i++ {
		lines = append(lines, i)
	}
	return lines
}

type PositionRange struct {
	Line        int // 1-indexed
	FirstColumn int // 1-indexed
	LastColumn  int // 1-indexed
}

type PositionRanges []PositionRange

func (prs PositionRanges) Len() (l int) {
	for _, pr := range prs {
		l += pr.LastColumn - pr.FirstColumn + 1
	}
	return l
}

func (prs PositionRanges) Lines() (lr LineRange) {
	for i, pr := range prs {
		if i == 0 {
			lr.First = pr.Line
			lr.Last = pr.Line
			continue
		}
		lr.First = min(lr.First, pr.Line)
		lr.Last = max(lr.Last, pr.Line)
	}
	return lr
}

func (prs PositionRanges) AddOffset(line, column int) {
	for i := range prs {
		prs[i].Line += line
		prs[i].FirstColumn += column
		prs[i].LastColumn += column
	}
}

func appendPosition(src PositionRanges, line, column int) PositionRanges {
	size := len(src)

	if size == 0 {
		return append(src, PositionRange{
			Line:        line,
			FirstColumn: column,
			LastColumn:  column,
		})
	}

	if src[size-1].Line == line && src[size-1].LastColumn+1 == column {
		src[size-1].LastColumn = column
		return src
	}

	return append(src, PositionRange{
		Line:        line,
		FirstColumn: column,
		LastColumn:  column,
	})
}

func NewPositionRange(lines []string, val *yaml.Node, minColumn int) PositionRanges {
	offsets := make(PositionRanges, 0, 10)

	if len(val.Value) == 0 {
		return PositionRanges{
			{Line: val.Line, FirstColumn: val.Column, LastColumn: val.Column},
		}
	}

	var needIndex, lineSpaces, valSpaces int
	need := val.Value[needIndex]
	lineIndex := val.Line
	columnIndex := val.Column

	for lineIndex <= len(lines) {
		// Append new line but only if we already have any tokens.
		if len(offsets) > 0 {
			offsets = appendPosition(offsets, lineIndex-1, len(lines[lineIndex-2])+1)
		}

		if len(lines[lineIndex-1]) == 0 {
			goto NEXT
		}

		columnIndex = min(len(lines[lineIndex-1]), columnIndex)

		lineSpaces = countLeadingSpace(lines[lineIndex-1][columnIndex-1:])
		valSpaces = countLeadingSpace(val.Value[needIndex:])
		if lineSpaces > valSpaces {
			columnIndex += lineSpaces - valSpaces
		}

		for gotIndex, got := range []byte(lines[lineIndex-1][columnIndex-1:]) {
			if need == got {
				offsets = appendPosition(offsets, lineIndex, columnIndex+gotIndex)
				needIndex++
				if needIndex >= len(val.Value) {
					goto END
				}
				need = val.Value[needIndex]
			}
		}

	NEXT:
		lineIndex++
		columnIndex = minColumn

		if need == ' ' || need == '\n' {
			needIndex++
			if needIndex >= len(val.Value) {
				goto END
			}
			need = val.Value[needIndex]
		}
	}

END:
	return offsets
}

func countLeadingSpace(line string) (i int) {
	for _, r := range line {
		if r != ' ' {
			return i
		}
		i++
	}
	return i
}

func readRange(firstColumn, lastColumn int, prs PositionRanges) PositionRanges {
	out := make(PositionRanges, 0, len(prs))

	var index int
	for _, pr := range prs {
		for j := pr.FirstColumn; j <= pr.LastColumn; j++ {
			index++
			if index >= firstColumn && index <= lastColumn {
				out = appendPosition(out, pr.Line, j)
			}
		}
	}

	return out
}
