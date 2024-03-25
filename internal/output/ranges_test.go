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
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			output := output.FormatLineRangeString(tc.lines)

			require.Equal(t, tc.output, output, "FormatLineRangeString() returned wrong output")
		})
	}
}
