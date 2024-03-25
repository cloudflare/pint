package output_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/output"
)

func TestHumanizeDuration(t *testing.T) {
	type testCaseT struct {
		output string
		input  time.Duration
	}

	testCases := []testCaseT{
		{
			input:  0,
			output: "0",
		},
		{
			input:  time.Microsecond * 3,
			output: "0",
		},
		{
			input:  time.Millisecond * 542,
			output: "542ms",
		},
		{
			input:  time.Second * 9,
			output: "9s",
		},
		{
			input:  time.Minute * 59,
			output: "59m",
		},
		{
			input:  time.Hour * 23,
			output: "23h",
		},
		{
			input:  time.Hour * 24 * 6,
			output: "6d",
		},
		{
			input:  time.Hour * 24 * 7 * 14,
			output: "14w",
		},
		{
			input:  (time.Hour * (24*7*14 + 24*6 + 3)),
			output: "14w6d3h",
		},
		{
			input:  (time.Hour * (24*7*14 + 24*6 + 3)) + time.Minute*33 + time.Second*3 + time.Millisecond + 999 + time.Microsecond*5,
			output: "14w6d3h33m3s1ms",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.input.String(), func(t *testing.T) {
			output := output.HumanizeDuration(tc.input)
			require.Equal(t, tc.output, output)
		})
	}
}

func TestHumanizeBytes(t *testing.T) {
	type testCaseT struct {
		output string
		input  int
	}

	testCases := []testCaseT{
		{
			input:  0,
			output: "0B",
		},
		{
			input:  10,
			output: "10B",
		},
		{
			input:  1024,
			output: "1.0KiB",
		},
		{
			input:  100000,
			output: "97.7KiB",
		},
		{
			input:  1024 * 4096,
			output: "4.0MiB",
		},
		{
			input:  1024*1024*1024 + 5,
			output: "1.0GiB",
		},
		{
			input:  1024*1024*1024 + 500000000,
			output: "1.5GiB",
		},
		{
			input:  1024 * 1024 * 1024 * 1024,
			output: "1.0TiB",
		},
	}

	for _, tc := range testCases {
		t.Run(strconv.Itoa(tc.input), func(t *testing.T) {
			output := output.HumanizeBytes(tc.input)
			require.Equal(t, tc.output, output)
		})
	}
}
