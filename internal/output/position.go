package output

import (
	"fmt"
	"strconv"

	"gopkg.in/yaml.v3"
)

type position struct {
	Line   int // 1-indexed
	Column int // 1-indexed
}

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

func (prs PositionRanges) Len() (l int) { // FIXME remove
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

func (prs PositionRanges) AddOffset(line, column int) PositionRanges {
	dst := make(PositionRanges, 0, len(prs))
	for _, pr := range prs {
		dst = append(dst, PositionRange{
			Line:        pr.Line + line,
			FirstColumn: pr.FirstColumn + column,
			LastColumn:  pr.LastColumn + column,
		})
	}
	return dst
}

func NewPositionRange(lines []string, val *yaml.Node, minColumn int) PositionRanges {
	offsets := make([]position, 0, len(val.Value))

	if len(val.Value) == 0 {
		return PositionRanges{
			{Line: val.Line, FirstColumn: val.Column, LastColumn: val.Column},
		}
	}

	var needIndex, lineSpaces, valSpaces int
	need := val.Value[needIndex]
	lineIndex := val.Line
	columnIndex := val.Column

	for {
		if lineIndex > len(lines) {
			break
		}

		// Append new line but only if we already have any tokens.
		if len(offsets) > 0 {
			offsets = append(offsets, position{
				Line:   lineIndex - 1,
				Column: len(lines[lineIndex-2]) + 1,
			})
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
				offsets = append(offsets, position{
					Line:   lineIndex,
					Column: columnIndex + gotIndex,
				})

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
	return mergePositions(offsets)
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

func mergePositions(input []position) (prs PositionRanges) {
	var last PositionRange
	for _, p := range input {
		if last.Line == 0 {
			last.Line = p.Line
			last.FirstColumn = p.Column
			last.LastColumn = p.Column
		}
		if p.Line != last.Line {
			prs = append(prs, last)
			last.Line = p.Line
			last.FirstColumn = p.Column
			last.LastColumn = p.Column
			continue
		}
		if last.LastColumn+1 == p.Column {
			last.LastColumn = p.Column
		}
	}
	prs = append(prs, last)
	return prs
}

func readRange(firstColumn, lastColumn int, in PositionRanges) PositionRanges {
	translated := rangesToOffsets(in)
	if firstColumn >= len(translated) {
		return nil
	}
	return mergePositions(translated[firstColumn-1 : min(lastColumn, len(translated))])
}

func rangesToOffsets(prs PositionRanges) (offsets []position) {
	for _, pr := range prs {
		for j := pr.FirstColumn; j <= pr.LastColumn; j++ {
			offsets = append(offsets, position{
				Line:   pr.Line,
				Column: j,
			})
		}
	}
	return offsets
}
