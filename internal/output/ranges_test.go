package output_test

import (
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/cloudflare/pint/internal/output"
)

func TestFormatLineRangeString(t *testing.T) {
	type testCaseT struct {
		lines  []int
		output string
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

			if diff := cmp.Diff(tc.output, output); diff != "" {
				t.Errorf("FormatLineRangeString() returned wrong output (-want +got):\n%s", diff)
				return
			}
		})
	}
}
