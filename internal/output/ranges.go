package output

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

func FormatLineRangeString(lines []int) string {
	ls := make([]int, len(lines))
	copy(ls, lines)
	slices.Sort(ls)

	var ranges []string
	start := -1
	end := -1
	for _, l := range ls {
		switch {
		case start < 0:
			start = l
			end = l
		case l == end+1:
			end = l
		default:
			if start > 0 && end > 0 {
				ranges = append(ranges, printRange(start, end))
			}
			start = l
			end = l
		}
	}
	if start > 0 && end > 0 {
		ranges = append(ranges, printRange(start, end))
	}

	return strings.Join(ranges, " ")
}

func printRange(start, end int) string {
	if start == end {
		return strconv.Itoa(start)
	}
	return fmt.Sprintf("%d-%d", start, end)
}
