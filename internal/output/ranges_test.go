package output_test

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/output"
)

func TestFormatLineRangeString(t *testing.T) {
	type testCaseT struct {
		output string
		lines  []int
	}

	testCases := []testCaseT{
		{
			lines:  []int{1, 2, 3},
			output: "1-3",
		},
		{
			lines:  []int{2, 1, 3},
			output: "1-3",
		},
		{
			lines:  []int{1, 3, 5},
			output: "1 3 5",
		},
		{
			lines:  []int{13, 12, 3, 5, 4, 7},
			output: "3-5 7 12-13",
		},
		{
			lines:  []int{},
			output: "",
		},
		{
			lines:  []int{1},
			output: "1",
		},
		{
			lines:  []int{-1, 0, 1},
			output: "",
		},
		{
			lines:  []int{1, 2, 4, 5, 6, 8, 10, 11, 12},
			output: "1-2 4-6 8 10-12",
		},
		{
			lines:  []int{100, 101, 102, 200},
			output: "100-102 200",
		},
		{
			lines:  []int{5, 5, 5},
			output: "5 5 5",
		},
		{
			lines:  []int{10, 1, 5, 3, 2},
			output: "1-3 5 10",
		},
		{
			lines:  []int{1, 3, 2, 5, 4},
			output: "1-5",
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			output := output.FormatLineRangeString(tc.lines)

			require.Equal(t, tc.output, output, "FormatLineRangeString() returned wrong output")
		})
	}
}
