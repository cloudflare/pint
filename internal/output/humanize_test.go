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
		{
			input:  time.Minute*3 + time.Second*45,
			output: "3m45s",
		},
		{
			input:  time.Second*30 + time.Millisecond*250,
			output: "30s250ms",
		},
		{
			input:  time.Hour*24*7 + time.Millisecond*1,
			output: "1w1ms",
		},
		{
			input:  time.Millisecond,
			output: "1ms",
		},
		{
			input:  time.Hour,
			output: "1h",
		},
		{
			input:  time.Minute,
			output: "1m",
		},
		{
			input:  time.Second,
			output: "1s",
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
		{
			input:  1024 * 1024 * 1024 * 1024 * 1024,
			output: "1.0PiB",
		},
		{
			input:  1024 * 1024 * 1024 * 1024 * 1024 * 1024,
			output: "1.0EiB",
		},
		{
			input:  512,
			output: "512B",
		},
		{
			input:  1536,
			output: "1.5KiB",
		},
	}

	for _, tc := range testCases {
		t.Run(strconv.Itoa(tc.input), func(t *testing.T) {
			output := output.HumanizeBytes(tc.input)
			require.Equal(t, tc.output, output)
		})
	}
}

func TestMaybeColor(t *testing.T) {
	type testCaseT struct {
		name     string
		input    string
		output   string
		color    output.Color
		disabled bool
	}

	testCases := []testCaseT{
		{
			name:     "color disabled",
			color:    output.Red,
			disabled: true,
			input:    "test string",
			output:   "test string",
		},
		{
			name:     "color enabled with red",
			color:    output.Red,
			disabled: false,
			input:    "error",
			output:   "\033[91merror\033[0m",
		},
		{
			name:     "color enabled with yellow",
			color:    output.Yellow,
			disabled: false,
			input:    "warning",
			output:   "\033[93mwarning\033[0m",
		},
		{
			name:     "color enabled with blue",
			color:    output.Blue,
			disabled: false,
			input:    "info",
			output:   "\033[94minfo\033[0m",
		},
		{
			name:     "color enabled with bold",
			color:    output.Bold,
			disabled: false,
			input:    "bold text",
			output:   "\033[1mbold text\033[0m",
		},
		{
			name:     "color enabled with none",
			color:    output.None,
			disabled: false,
			input:    "no color",
			output:   "\033[0mno color\033[0m",
		},
		{
			name:     "empty string with color",
			color:    output.Cyan,
			disabled: false,
			input:    "",
			output:   "\033[96m\033[0m",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := output.MaybeColor(tc.color, tc.disabled, tc.input)
			require.Equal(t, tc.output, result)
		})
	}
}
