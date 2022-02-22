package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDurationMatch(t *testing.T) {
	type testCaseT struct {
		input  string
		output durationMatch
		err    string
	}

	testCases := []testCaseT{
		{
			input:  "=5m",
			output: durationMatch{},
			err:    `not a valid duration string: "=5m"`,
		},
		{
			input:  "! 5m",
			output: durationMatch{},
			err:    "unknown duration match operation: !",
		},
		{
			input: "= 3s ",
			err:   `not a valid duration string: "3s "`,
		},
		{
			input: "= = 3s",
			err:   `not a valid duration string: "= 3s"`,
		},
		{
			input: "5m",
			output: durationMatch{
				op:  opEqual,
				dur: time.Duration(time.Minute * 5),
			},
		},
		{
			input: "= 1w",
			output: durationMatch{
				op:  opEqual,
				dur: time.Duration(time.Hour * 24 * 7),
			},
		},
		{
			input: "!= 0",
			output: durationMatch{
				op:  opNotEqual,
				dur: time.Duration(0),
			},
		},
		{
			input: "!= 10w",
			output: durationMatch{
				op:  opNotEqual,
				dur: time.Duration(time.Hour * 24 * 7 * 10),
			},
		},
		{
			input: "> 5m",
			output: durationMatch{
				op:  opMore,
				dur: time.Duration(time.Minute * 5),
			},
		},
		{
			input: "< 1s",
			output: durationMatch{
				op:  opLess,
				dur: time.Duration(time.Second),
			},
		},
		{
			input: "<= 0s",
			output: durationMatch{
				op:  opLessEqual,
				dur: time.Duration(0),
			},
		},
		{
			input: ">= 25h",
			output: durationMatch{
				op:  opMoreEqual,
				dur: time.Duration(time.Hour * 25),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			assert := assert.New(t)
			output, err := parseDurationMatch(tc.input)
			if tc.err == "" {
				assert.NoError(err)
				assert.Equal(tc.output, output)
			} else {
				assert.EqualError(err, tc.err)
			}
		})
	}
}

func TestDurationMatchIsMatch(t *testing.T) {
	type testCaseT struct {
		input    string
		duration time.Duration
		isMatch  bool
	}

	testCases := []testCaseT{
		{
			input:    "240s",
			duration: time.Duration(time.Minute * 4),
			isMatch:  true,
		},
		{
			input:    "3m59s",
			duration: time.Duration(time.Minute * 4),
			isMatch:  false,
		},
		{
			input:    "= 0s",
			duration: time.Duration(0),
			isMatch:  true,
		},
		{
			input:    "= 30s",
			duration: time.Duration(time.Second),
			isMatch:  false,
		},
		{
			input:    "!= 4m",
			duration: time.Duration(time.Minute * 5),
			isMatch:  true,
		},
		{
			input:    "!= 1s",
			duration: time.Duration(time.Second),
			isMatch:  false,
		},
		{
			input:    "< 4m",
			duration: time.Duration(time.Minute * 3),
			isMatch:  true,
		},
		{
			input:    "< 59s",
			duration: time.Duration(time.Minute),
			isMatch:  false,
		},
		{
			input:    "<= 4m",
			duration: time.Duration(time.Minute * 4),
			isMatch:  true,
		},
		{
			input:    "<= 4m1s",
			duration: time.Duration(time.Minute * 4),
			isMatch:  true,
		},
		{
			input:    "<= 59s",
			duration: time.Duration(time.Minute),
			isMatch:  false,
		},
		{
			input:    ">= 4m",
			duration: time.Duration(time.Minute * 4),
			isMatch:  true,
		},
		{
			input:    ">= 3m59s",
			duration: time.Duration(time.Minute * 4),
			isMatch:  true,
		},
		{
			input:    ">= 61s",
			duration: time.Duration(time.Minute),
			isMatch:  false,
		},
		{
			input:    "> 0s",
			duration: time.Duration(time.Microsecond),
			isMatch:  true,
		},
		{
			input:    "> 1ms",
			duration: time.Duration(time.Nanosecond),
			isMatch:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			assert := assert.New(t)
			d, err := parseDurationMatch(tc.input)
			assert.NoError(err)
			isMatch := d.isMatch(tc.duration)
			assert.Equal(tc.isMatch, isMatch, "input=%q duration=%s", tc.input, tc.duration)
		})
	}
}
