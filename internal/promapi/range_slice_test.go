package promapi

import (
	"fmt"
	"strconv"
	"strings"
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
		output     []TimeRange
	}

	timeParse := func(s string) time.Time {
		v, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}

	printRange := func(tr []TimeRange) string {
		var buf strings.Builder
		for _, r := range tr {
			buf.WriteString(fmt.Sprintf("%s - %s\n", r.Start.Format(time.RFC3339), r.End.Format(time.RFC3339)))
		}
		return buf.String()
	}

	testCases := []testCaseT{
		{
			start:      timeParse("2022-01-01T23:00:00Z"),
			end:        timeParse("2022-01-02T01:00:00Z"),
			resolution: time.Minute * 5,
			sliceSize:  time.Hour * 2,
			output: []TimeRange{
				{
					Start: timeParse("2022-01-01T22:00:00Z"),
					End:   timeParse("2022-01-01T23:59:59Z"),
				},
				{
					Start: timeParse("2022-01-02T00:00:00Z"),
					End:   timeParse("2022-01-02T01:00:00Z"),
				},
			},
		},
		{
			start:      timeParse("2022-01-01T00:00:00Z"),
			end:        timeParse("2022-01-01T02:00:00Z"),
			resolution: time.Minute * 5,
			sliceSize:  time.Hour * 2,
			output: []TimeRange{
				{
					Start: timeParse("2022-01-01T00:00:00Z"),
					End:   timeParse("2022-01-01T02:00:00Z"),
				},
			},
		},
		{
			start:      timeParse("2022-01-01T01:00:00Z"),
			end:        timeParse("2022-01-01T01:30:00Z"),
			resolution: time.Minute * 5,
			sliceSize:  time.Hour,
			output: []TimeRange{
				{
					Start: timeParse("2022-01-01T01:00:00Z"),
					End:   timeParse("2022-01-01T01:30:00Z"),
				},
			},
		},
		{
			start:      timeParse("2022-01-01T01:00:00Z"),
			end:        timeParse("2022-01-01T01:30:00Z"),
			resolution: time.Minute * 5,
			sliceSize:  time.Hour * 2,
			output: []TimeRange{
				{
					Start: timeParse("2022-01-01T00:00:00Z"),
					End:   timeParse("2022-01-01T01:30:00Z"),
				},
			},
		},
		{
			start:      timeParse("2022-01-01T00:00:00Z"),
			end:        timeParse("2022-01-01T11:00:00Z"),
			resolution: time.Minute * 5,
			sliceSize:  time.Hour * 2,
			output: []TimeRange{
				{
					Start: timeParse("2022-01-01T00:00:00Z"),
					End:   timeParse("2022-01-01T01:59:59Z"),
				},
				{
					Start: timeParse("2022-01-01T02:00:00Z"),
					End:   timeParse("2022-01-01T03:59:59Z"),
				},
				{
					Start: timeParse("2022-01-01T04:00:00Z"),
					End:   timeParse("2022-01-01T05:59:59Z"),
				},
				{
					Start: timeParse("2022-01-01T06:00:00Z"),
					End:   timeParse("2022-01-01T07:59:59Z"),
				},
				{
					Start: timeParse("2022-01-01T08:00:00Z"),
					End:   timeParse("2022-01-01T09:59:59Z"),
				},
				{
					Start: timeParse("2022-01-01T10:00:00Z"),
					End:   timeParse("2022-01-01T11:00:00Z"),
				},
			},
		},
		{
			start:      timeParse("2022-01-01T00:59:00Z"),
			end:        timeParse("2022-01-01T00:59:59Z"),
			resolution: time.Minute * 5,
			sliceSize:  time.Hour * 2,
			output: []TimeRange{
				{
					Start: timeParse("2022-01-01T00:59:00Z"),
					End:   timeParse("2022-01-01T00:59:59Z"),
				},
			},
		},
		{
			start:      timeParse("2022-01-01T00:30:00Z"),
			end:        timeParse("2022-01-01T03:30:00Z"),
			resolution: time.Minute * 5,
			sliceSize:  time.Hour * 2,
			output: []TimeRange{
				{
					Start: timeParse("2022-01-01T00:00:00Z"),
					End:   timeParse("2022-01-01T01:59:59Z"),
				},
				{
					Start: timeParse("2022-01-01T02:00:00Z"),
					End:   timeParse("2022-01-01T03:30:00Z"),
				},
			},
		},
		{
			start:      timeParse("2022-01-01T23:59:00Z"),
			end:        timeParse("2022-01-02T00:30:00Z"),
			resolution: time.Minute * 5,
			sliceSize:  time.Hour * 2,
			output: []TimeRange{
				{
					Start: timeParse("2022-01-01T22:00:00Z"),
					End:   timeParse("2022-01-01T23:59:59Z"),
				},
				{
					Start: timeParse("2022-01-02T00:00:00Z"),
					End:   timeParse("2022-01-02T00:30:00Z"),
				},
			},
		},
		{
			start:      timeParse("2022-01-01T23:45:00Z"),
			end:        timeParse("2022-01-02T02:30:00Z"),
			resolution: time.Minute * 5,
			sliceSize:  time.Hour * 2,
			output: []TimeRange{
				{
					Start: timeParse("2022-01-01T22:00:00Z"),
					End:   timeParse("2022-01-01T23:59:59Z"),
				},
				{
					Start: timeParse("2022-01-02T00:00:00Z"),
					End:   timeParse("2022-01-02T01:59:59Z"),
				},
				{
					Start: timeParse("2022-01-02T02:00:00Z"),
					End:   timeParse("2022-01-02T02:30:00Z"),
				},
			},
		},
		{
			start:      timeParse("2022-01-01T11:11:11Z"),
			end:        timeParse("2022-01-01T13:11:11Z"),
			resolution: time.Minute * 5,
			sliceSize:  time.Hour * 2,
			output: []TimeRange{
				{
					Start: timeParse("2022-01-01T10:00:00Z"),
					End:   timeParse("2022-01-01T11:59:59Z"),
				},
				{
					Start: timeParse("2022-01-01T12:00:00Z"),
					End:   timeParse("2022-01-01T13:11:11Z"),
				},
			},
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			output := sliceRange(tc.start, tc.end, tc.resolution, tc.sliceSize)
			require.Equal(t, printRange(tc.output), printRange(output))
		})
	}
}
