package promapi

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSliceRange(t *testing.T) {
	type testCaseT struct {
		start      time.Time
		end        time.Time
		resolution time.Duration
		sliceSize  time.Duration
		output     []timeRange
	}

	timeParse := func(s string) time.Time {
		v, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}

	testCases := []testCaseT{
		{
			start:      timeParse("2022-01-01T00:00:00Z"),
			end:        timeParse("2022-01-01T02:00:00Z"),
			resolution: time.Minute * 5,
			sliceSize:  time.Hour * 2,
			output: []timeRange{
				{
					start: timeParse("2022-01-01T00:00:00Z"),
					end:   timeParse("2022-01-01T02:00:00Z"),
				},
			},
		},
		{
			start:      timeParse("2022-01-01T00:00:00Z"),
			end:        timeParse("2022-01-01T11:00:00Z"),
			resolution: time.Minute * 5,
			sliceSize:  time.Hour * 2,
			output: []timeRange{
				{
					start: timeParse("2022-01-01T00:00:00Z"),
					end:   timeParse("2022-01-01T01:55:00Z"),
				},
				{
					start: timeParse("2022-01-01T02:00:00Z"),
					end:   timeParse("2022-01-01T03:55:00Z"),
				},
				{
					start: timeParse("2022-01-01T04:00:00Z"),
					end:   timeParse("2022-01-01T05:55:00Z"),
				},
				{
					start: timeParse("2022-01-01T06:00:00Z"),
					end:   timeParse("2022-01-01T07:55:00Z"),
				},
				{
					start: timeParse("2022-01-01T08:00:00Z"),
					end:   timeParse("2022-01-01T09:55:00Z"),
				},
				{
					start: timeParse("2022-01-01T10:00:00Z"),
					end:   timeParse("2022-01-01T11:00:00Z"),
				},
			},
		},
		{
			start:      timeParse("2022-01-01T00:30:00Z"),
			end:        timeParse("2022-01-01T03:30:00Z"),
			resolution: time.Minute * 5,
			sliceSize:  time.Hour * 2,
			output: []timeRange{
				{
					start: timeParse("2022-01-01T00:30:00Z"),
					end:   timeParse("2022-01-01T02:25:00Z"),
				},
				{
					start: timeParse("2022-01-01T02:30:00Z"),
					end:   timeParse("2022-01-01T03:30:00Z"),
				},
			},
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			output := sliceRange(tc.start, tc.end, tc.resolution, tc.sliceSize)
			require.Equal(t, tc.output, output)
		})
	}
}
