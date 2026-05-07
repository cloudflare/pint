package promapi_test

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cloudflare/pint/internal/promapi"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"
)

func TestAppendSampleToRanges(t *testing.T) {
	type testCaseT struct {
		in      promapi.MetricTimeRanges
		samples []model.SampleStream
		out     promapi.MetricTimeRanges
		step    time.Duration
	}

	timeParse := func(s string) time.Time {
		v, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatal(err)
		}
		return v.UTC()
	}

	printRange := func(tr []promapi.MetricTimeRange) string {
		var buf strings.Builder
		for _, r := range tr {
			fmt.Fprintf(&buf, "%s %s - %s\n", r.Labels, r.Start.UTC().Format(time.RFC3339), r.End.UTC().Format(time.RFC3339))
		}
		return buf.String()
	}

	testCases := []testCaseT{
		{
			in: nil,
			samples: []model.SampleStream{
				{
					Metric: model.Metric{"instance": "1"},
					Values: generateSamples(timeParse("2022-06-14T00:00:00Z"), timeParse("2022-06-14T02:55:00Z"), time.Minute*5),
				},
				{
					Metric: model.Metric{"instance": "2"},
					Values: generateSamples(timeParse("2022-06-14T00:00:00Z"), timeParse("2022-06-14T02:55:00Z"), time.Minute*5),
				},
				{
					Metric: model.Metric{"instance": "3"},
					Values: generateSamples(timeParse("2022-06-14T00:00:00Z"), timeParse("2022-06-14T02:55:00Z"), time.Minute*5),
				},
			},
			step: time.Minute * 5,
			out: []promapi.MetricTimeRange{
				{
					Fingerprint: labels.FromStrings("instance", "1").Hash(),
					Labels:      labels.FromStrings("instance", "1"),
					Start:       timeParse("2022-06-14T00:00:00Z"),
					End:         timeParse("2022-06-14T02:55:00Z"),
				},
				{
					Fingerprint: labels.FromStrings("instance", "2").Hash(),
					Labels:      labels.FromStrings("instance", "2"),
					Start:       timeParse("2022-06-14T00:00:00Z"),
					End:         timeParse("2022-06-14T02:55:00Z"),
				},
				{
					Fingerprint: labels.FromStrings("instance", "3").Hash(),
					Labels:      labels.FromStrings("instance", "3"),
					Start:       timeParse("2022-06-14T00:00:00Z"),
					End:         timeParse("2022-06-14T02:55:00Z"),
				},
			},
		},
		{
			in: nil,
			samples: []model.SampleStream{
				{
					Metric: model.Metric{"instance": "1"},
					Values: generateSamples(timeParse("2022-06-14T00:00:00Z"), timeParse("2022-06-14T02:55:00Z"), time.Minute*5),
				},
				{
					Metric: model.Metric{"instance": "1", "job": "foo"},
					Values: generateSamples(timeParse("2022-06-14T00:00:00Z"), timeParse("2022-06-14T02:55:00Z"), time.Minute*5),
				},
				{
					Metric: model.Metric{"job": "bar"},
					Values: generateSamples(timeParse("2022-06-14T00:00:00Z"), timeParse("2022-06-14T02:55:00Z"), time.Minute*5),
				},
				{
					Metric: model.Metric{},
					Values: generateSamples(timeParse("2022-06-14T00:00:00Z"), timeParse("2022-06-14T02:55:00Z"), time.Minute*5),
				},
			},
			step: time.Minute * 5,
			out: []promapi.MetricTimeRange{
				{
					Fingerprint: labels.FromStrings().Hash(),
					Labels:      labels.FromStrings(),
					Start:       timeParse("2022-06-14T00:00:00Z"),
					End:         timeParse("2022-06-14T02:55:00Z"),
				},
				{
					Fingerprint: labels.FromStrings("instance", "1").Hash(),
					Labels:      labels.FromStrings("instance", "1"),
					Start:       timeParse("2022-06-14T00:00:00Z"),
					End:         timeParse("2022-06-14T02:55:00Z"),
				},
				{
					Fingerprint: labels.FromStrings("instance", "1", "job", "foo").Hash(),
					Labels:      labels.FromStrings("instance", "1", "job", "foo"),
					Start:       timeParse("2022-06-14T00:00:00Z"),
					End:         timeParse("2022-06-14T02:55:00Z"),
				},
				{
					Fingerprint: labels.FromStrings("job", "bar").Hash(),
					Labels:      labels.FromStrings("job", "bar"),
					Start:       timeParse("2022-06-14T00:00:00Z"),
					End:         timeParse("2022-06-14T02:55:00Z"),
				},
			},
		},
		{
			in: []promapi.MetricTimeRange{
				{
					Fingerprint: labels.FromStrings("instance", "1").Hash(),
					Labels:      labels.FromStrings("instance", "1"),
					Start:       timeParse("2022-06-14T00:00:00Z"),
					End:         timeParse("2022-06-14T02:55:00Z"),
				},
				{
					Fingerprint: labels.FromStrings("instance", "3").Hash(),
					Labels:      labels.FromStrings("instance", "3"),
					Start:       timeParse("2022-06-14T00:00:00Z"),
					End:         timeParse("2022-06-14T02:55:00Z"),
				},
				{
					Fingerprint: labels.FromStrings("instance", "2").Hash(),
					Labels:      labels.FromStrings("instance", "2"),
					Start:       timeParse("2022-06-14T00:00:00Z"),
					End:         timeParse("2022-06-14T02:55:00Z"),
				},
			},
			samples: []model.SampleStream{
				{
					Metric: model.Metric{"instance": "1"},
					Values: generateSamples(timeParse("2022-06-14T03:00:00Z"), timeParse("2022-06-14T03:55:00Z"), time.Minute*5),
				},
				{
					Metric: model.Metric{"instance": "1"},
					Values: generateSamples(timeParse("2022-06-13T10:00:00Z"), timeParse("2022-06-13T12:55:00Z"), time.Minute*5),
				},
				{
					Metric: model.Metric{"instance": "2"},
					Values: generateSamples(timeParse("2022-06-15T10:00:00Z"), timeParse("2022-06-15T12:55:00Z"), time.Minute*5),
				},
				{
					Metric: model.Metric{"instance": "1"},
					Values: generateSamples(timeParse("2022-06-13T23:00:00Z"), timeParse("2022-06-13T23:55:00Z"), time.Minute*5),
				},
			},
			step: time.Minute * 5,
			out: []promapi.MetricTimeRange{
				{
					Fingerprint: labels.FromStrings("instance", "1").Hash(),
					Labels:      labels.FromStrings("instance", "1"),
					Start:       timeParse("2022-06-13T10:00:00Z"),
					End:         timeParse("2022-06-13T12:55:00Z"),
				},
				{
					Fingerprint: labels.FromStrings("instance", "1").Hash(),
					Labels:      labels.FromStrings("instance", "1"),
					Start:       timeParse("2022-06-13T23:00:00Z"),
					End:         timeParse("2022-06-14T03:55:00Z"),
				},
				{
					Fingerprint: labels.FromStrings("instance", "2").Hash(),
					Labels:      labels.FromStrings("instance", "2"),
					Start:       timeParse("2022-06-14T00:00:00Z"),
					End:         timeParse("2022-06-14T02:55:00Z"),
				},
				{
					Fingerprint: labels.FromStrings("instance", "2").Hash(),
					Labels:      labels.FromStrings("instance", "2"),
					Start:       timeParse("2022-06-15T10:00:00Z"),
					End:         timeParse("2022-06-15T12:55:00Z"),
				},
				{
					Fingerprint: labels.FromStrings("instance", "3").Hash(),
					Labels:      labels.FromStrings("instance", "3"),
					Start:       timeParse("2022-06-14T00:00:00Z"),
					End:         timeParse("2022-06-14T02:55:00Z"),
				},
			},
		},
		{
			in: nil,
			samples: []model.SampleStream{
				{
					Metric: model.Metric{"instance": "1"},
					Values: generateSamples(timeParse("2022-06-14T00:00:00Z"), timeParse("2022-06-14T02:55:00Z"), time.Minute*5),
				},
				{
					Metric: model.Metric{"instance": "1"},
					Values: generateSamples(timeParse("2022-06-14T02:00:00Z"), timeParse("2022-06-14T05:55:00Z"), time.Minute*5),
				},
				{
					Metric: model.Metric{"instance": "1"},
					Values: generateSamples(timeParse("2022-06-14T03:00:00Z"), timeParse("2022-06-14T07:55:00Z"), time.Minute*5),
				},
			},
			step: time.Minute * 5,
			out: []promapi.MetricTimeRange{
				{
					Fingerprint: labels.FromStrings("instance", "1").Hash(),
					Labels:      labels.FromStrings("instance", "1"),
					Start:       timeParse("2022-06-14T00:00:00Z"),
					End:         timeParse("2022-06-14T07:55:00Z"),
				},
			},
		},
		{
			in: nil,
			samples: []model.SampleStream{
				{
					Metric: model.Metric{"instance": "1"},
					Values: generateSamples(timeParse("2022-10-27T09:14:59Z"), timeParse("2022-10-27T09:20:59Z"), time.Minute),
				},
				{
					Metric: model.Metric{"instance": "1"},
					Values: generateSamples(timeParse("2022-10-27T10:14:59Z"), timeParse("2022-10-27T10:20:59Z"), time.Minute),
				},
				{
					Metric: model.Metric{"instance": "1"},
					Values: generateSamples(timeParse("2022-10-27T11:14:59Z"), timeParse("2022-10-27T11:15:59Z"), time.Minute),
				},
				{
					Metric: model.Metric{"instance": "1"},
					Values: generateSamples(timeParse("2022-10-27T12:14:59Z"), timeParse("2022-10-27T12:30:59Z"), time.Minute),
				},
				{
					Metric: model.Metric{"instance": "1"},
					Values: generateSamples(timeParse("2022-10-27T13:14:59Z"), timeParse("2022-10-27T13:50:59Z"), time.Minute),
				},
				{
					Metric: model.Metric{"instance": "1"},
					Values: generateSamples(timeParse("2022-10-27T14:14:59Z"), timeParse("2022-10-27T14:50:59Z"), time.Minute),
				},
				{
					Metric: model.Metric{"instance": "1"},
					Values: generateSamples(timeParse("2022-10-27T23:14:59Z"), timeParse("2022-10-28T01:14:59Z"), time.Minute),
				},
			},
			step: time.Minute,
			out: []promapi.MetricTimeRange{
				{
					Fingerprint: labels.FromStrings("instance", "1").Hash(),
					Labels:      labels.FromStrings("instance", "1"),
					Start:       timeParse("2022-10-27T09:14:59Z"),
					End:         timeParse("2022-10-27T09:20:59Z"),
				},
				{
					Fingerprint: labels.FromStrings("instance", "1").Hash(),
					Labels:      labels.FromStrings("instance", "1"),
					Start:       timeParse("2022-10-27T10:14:59Z"),
					End:         timeParse("2022-10-27T10:20:59Z"),
				},
				{
					Fingerprint: labels.FromStrings("instance", "1").Hash(),
					Labels:      labels.FromStrings("instance", "1"),
					Start:       timeParse("2022-10-27T11:14:59Z"),
					End:         timeParse("2022-10-27T11:15:59Z"),
				},
				{
					Fingerprint: labels.FromStrings("instance", "1").Hash(),
					Labels:      labels.FromStrings("instance", "1"),
					Start:       timeParse("2022-10-27T12:14:59Z"),
					End:         timeParse("2022-10-27T12:30:59Z"),
				},
				{
					Fingerprint: labels.FromStrings("instance", "1").Hash(),
					Labels:      labels.FromStrings("instance", "1"),
					Start:       timeParse("2022-10-27T13:14:59Z"),
					End:         timeParse("2022-10-27T13:50:59Z"),
				},
				{
					Fingerprint: labels.FromStrings("instance", "1").Hash(),
					Labels:      labels.FromStrings("instance", "1"),
					Start:       timeParse("2022-10-27T14:14:59Z"),
					End:         timeParse("2022-10-27T14:50:59Z"),
				},
				{
					Fingerprint: labels.FromStrings("instance", "1").Hash(),
					Labels:      labels.FromStrings("instance", "1"),
					Start:       timeParse("2022-10-27T23:14:59Z"),
					End:         timeParse("2022-10-28T01:14:59Z"),
				},
			},
		},
		{
			in: nil,
			samples: []model.SampleStream{
				{
					Metric: model.Metric{"instance": "1"},
					Values: generateSamples(timeParse("2022-06-14T00:00:00Z"), timeParse("2022-06-14T00:00:00Z"), time.Minute*5),
				},
			},
			step: time.Minute * 5,
			out: []promapi.MetricTimeRange{
				{
					Fingerprint: labels.FromStrings("instance", "1").Hash(),
					Labels:      labels.FromStrings("instance", "1"),
					Start:       timeParse("2022-06-14T00:00:00Z"),
					End:         timeParse("2022-06-14T00:00:00Z"),
				},
			},
		},
		{
			in: nil,
			samples: []model.SampleStream{
				{
					Metric: model.Metric{"instance": "1"},
					Values: generateSamples(timeParse("2022-06-14T00:00:00Z"), timeParse("2022-06-14T00:00:00Z"), time.Minute*5),
				},
				{
					Metric: model.Metric{"instance": "1"},
					Values: generateSamples(timeParse("2022-06-14T00:05:00Z"), timeParse("2022-06-14T00:05:00Z"), time.Minute*5),
				},
				{
					Metric: model.Metric{"instance": "1"},
					Values: generateSamples(timeParse("2022-06-14T00:15:00Z"), timeParse("2022-06-14T00:15:00Z"), time.Minute*5),
				},
			},
			step: time.Minute * 5,
			out: []promapi.MetricTimeRange{
				{
					Fingerprint: labels.FromStrings("instance", "1").Hash(),
					Labels:      labels.FromStrings("instance", "1"),
					Start:       timeParse("2022-06-14T00:00:00Z"),
					End:         timeParse("2022-06-14T00:05:00Z"),
				},
				{
					Fingerprint: labels.FromStrings("instance", "1").Hash(),
					Labels:      labels.FromStrings("instance", "1"),
					Start:       timeParse("2022-06-14T00:15:00Z"),
					End:         timeParse("2022-06-14T00:15:00Z"),
				},
			},
		},
		{
			in: nil,
			samples: []model.SampleStream{
				{
					Metric: model.Metric{"instance": "1"},
					Values: generateSamples(timeParse("2022-06-14T00:00:00Z"), timeParse("2022-06-14T05:00:00Z"), time.Minute*5),
				},
				{
					Metric: model.Metric{"instance": "1"},
					Values: generateSamples(timeParse("2022-06-14T05:10:00Z"), timeParse("2022-06-14T06:10:00Z"), time.Minute*5),
				},
				{
					Metric: model.Metric{"instance": "1"},
					Values: generateSamples(timeParse("2022-06-14T06:20:00Z"), timeParse("2022-06-14T06:20:00Z"), time.Minute*5),
				},
			},
			step: time.Minute * 5,
			out: []promapi.MetricTimeRange{
				{
					Fingerprint: labels.FromStrings("instance", "1").Hash(),
					Labels:      labels.FromStrings("instance", "1"),
					Start:       timeParse("2022-06-14T00:00:00Z"),
					End:         timeParse("2022-06-14T05:00:00Z"),
				},
				{
					Fingerprint: labels.FromStrings("instance", "1").Hash(),
					Labels:      labels.FromStrings("instance", "1"),
					Start:       timeParse("2022-06-14T05:10:00Z"),
					End:         timeParse("2022-06-14T06:10:00Z"),
				},
				{
					Fingerprint: labels.FromStrings("instance", "1").Hash(),
					Labels:      labels.FromStrings("instance", "1"),
					Start:       timeParse("2022-06-14T06:20:00Z"),
					End:         timeParse("2022-06-14T06:20:00Z"),
				},
			},
		},
		{
			in: nil,
			samples: []model.SampleStream{
				// #5. 06:00 - 06:25
				{
					Metric: model.Metric{"instance": "1"},
					Values: generateSamples(timeParse("2022-06-14T06:00:00Z"), timeParse("2022-06-14T06:20:00Z"), time.Minute*5),
				},
				// #1. 00:00 - 05:05
				{
					Metric: model.Metric{"instance": "1"},
					Values: generateSamples(timeParse("2022-06-14T00:00:00Z"), timeParse("2022-06-14T05:00:00Z"), time.Minute*5),
				},
				// #2. 05:10 - 05:15
				{
					Metric: model.Metric{"instance": "1"},
					Values: generateSamples(timeParse("2022-06-14T05:10:00Z"), timeParse("2022-06-14T05:10:00Z"), time.Minute*5),
				},
				// #6. 06:20 - 06:25
				{
					Metric: model.Metric{"instance": "1"},
					Values: generateSamples(timeParse("2022-06-14T06:20:00Z"), timeParse("2022-06-14T06:20:00Z"), time.Minute*5),
				},
				// #4. 05:25 - 06:00
				{
					Metric: model.Metric{"instance": "1"},
					Values: generateSamples(timeParse("2022-06-14T05:25:00Z"), timeParse("2022-06-14T05:55:00Z"), time.Minute*5),
				},
				// #3. 05:20 - 05:25
				{
					Metric: model.Metric{"instance": "1"},
					Values: generateSamples(timeParse("2022-06-14T05:20:00Z"), timeParse("2022-06-14T05:20:00Z"), time.Minute*5),
				},
			},
			step: time.Minute * 5,
			out: []promapi.MetricTimeRange{
				{
					Fingerprint: labels.FromStrings("instance", "1").Hash(),
					Labels:      labels.FromStrings("instance", "1"),
					Start:       timeParse("2022-06-14T00:00:00Z"),
					End:         timeParse("2022-06-14T05:00:00Z"),
				},
				{
					Fingerprint: labels.FromStrings("instance", "1").Hash(),
					Labels:      labels.FromStrings("instance", "1"),
					Start:       timeParse("2022-06-14T05:10:00Z"),
					End:         timeParse("2022-06-14T05:10:00Z"),
				},
				{
					Fingerprint: labels.FromStrings("instance", "1").Hash(),
					Labels:      labels.FromStrings("instance", "1"),
					Start:       timeParse("2022-06-14T05:20:00Z"),
					End:         timeParse("2022-06-14T06:20:00Z"),
				},
			},
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			for range 50 {
				tc := tc
				for _, s := range tc.samples {
					lset := promapi.MetricToLabels(s.Metric)
					tc.in = promapi.AppendSampleToRanges(tc.in, lset, s.Values, tc.step)
				}
				tc.in, _ = promapi.MergeRanges(tc.in, tc.step)
				slices.SortStableFunc(tc.in, promapi.CompareMetricTimeRanges)
				require.Equal(t, printRange(tc.out), printRange(tc.in))
			}
		})
	}
}

func TestMergeRanges(t *testing.T) {
	type testCaseT struct {
		in        promapi.MetricTimeRanges
		out       promapi.MetricTimeRanges
		step      time.Duration
		wasMerged bool
	}

	timeParse := func(s string) time.Time {
		v, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatal(err)
		}
		return v.UTC()
	}

	printRange := func(tr []promapi.MetricTimeRange) string {
		var buf strings.Builder
		for _, r := range tr {
			fmt.Fprintf(&buf, "%s %s - %s\n", r.Labels, r.Start.UTC().Format(time.RFC3339), r.End.UTC().Format(time.RFC3339))
		}
		return buf.String()
	}

	testCases := []testCaseT{
		{
			in:   nil,
			out:  nil,
			step: time.Minute,
		},
		{
			in:   promapi.MetricTimeRanges{},
			out:  promapi.MetricTimeRanges{},
			step: time.Minute,
		},
		{
			in: promapi.MetricTimeRanges{
				{Fingerprint: labels.EmptyLabels().Hash(), Labels: labels.EmptyLabels(), Start: timeParse("2022-10-20T00:00:44Z"), End: timeParse("2022-10-20T14:00:44Z")},
				{Fingerprint: labels.FromStrings("foo", "bar").Hash(), Labels: labels.EmptyLabels(), Start: timeParse("2022-10-20T00:00:44Z"), End: timeParse("2022-10-20T14:00:44Z")},
			},
			out: promapi.MetricTimeRanges{
				{Fingerprint: labels.EmptyLabels().Hash(), Labels: labels.EmptyLabels(), Start: timeParse("2022-10-20T00:00:44Z"), End: timeParse("2022-10-20T14:01:43Z")},
				{Fingerprint: labels.FromStrings("foo", "bar").Hash(), Labels: labels.EmptyLabels(), Start: timeParse("2022-10-20T00:00:44Z"), End: timeParse("2022-10-20T14:01:43Z")},
			},
			step: time.Minute,
		},
		{
			in: promapi.MetricTimeRanges{
				{Fingerprint: labels.EmptyLabels().Hash(), Labels: labels.EmptyLabels(), Start: timeParse("2022-10-20T00:00:44Z"), End: timeParse("2022-10-20T14:00:44Z")},
				{Fingerprint: labels.EmptyLabels().Hash(), Labels: labels.EmptyLabels(), Start: timeParse("2022-10-19T16:00:44Z"), End: timeParse("2022-10-19T20:00:44Z")},
				{Fingerprint: labels.EmptyLabels().Hash(), Labels: labels.EmptyLabels(), Start: timeParse("2022-10-19T14:00:44Z"), End: timeParse("2022-10-19T16:00:44Z")},
				{Fingerprint: labels.EmptyLabels().Hash(), Labels: labels.EmptyLabels(), Start: timeParse("2022-10-24T18:00:44Z"), End: timeParse("2022-10-25T22:00:44Z")},
				{Fingerprint: labels.EmptyLabels().Hash(), Labels: labels.EmptyLabels(), Start: timeParse("2022-10-19T22:00:44Z"), End: timeParse("2022-10-20T00:00:44Z")},
				{Fingerprint: labels.EmptyLabels().Hash(), Labels: labels.EmptyLabels(), Start: timeParse("2022-10-23T06:00:44Z"), End: timeParse("2022-10-23T14:00:44Z")},
				{Fingerprint: labels.EmptyLabels().Hash(), Labels: labels.EmptyLabels(), Start: timeParse("2022-10-22T14:00:44Z"), End: timeParse("2022-10-23T06:00:44Z")},
				{Fingerprint: labels.EmptyLabels().Hash(), Labels: labels.EmptyLabels(), Start: timeParse("2022-10-19T20:00:44Z"), End: timeParse("2022-10-19T22:00:44Z")},
				{Fingerprint: labels.EmptyLabels().Hash(), Labels: labels.EmptyLabels(), Start: timeParse("2022-10-19T12:00:44Z"), End: timeParse("2022-10-19T14:00:44Z")},
				{Fingerprint: labels.EmptyLabels().Hash(), Labels: labels.EmptyLabels(), Start: timeParse("2022-10-24T02:00:44Z"), End: timeParse("2022-10-24T10:00:44Z")},
				{Fingerprint: labels.EmptyLabels().Hash(), Labels: labels.EmptyLabels(), Start: timeParse("2022-10-22T12:00:44Z"), End: timeParse("2022-10-22T14:00:44Z")},
				{Fingerprint: labels.EmptyLabels().Hash(), Labels: labels.EmptyLabels(), Start: timeParse("2022-10-24T00:00:44Z"), End: timeParse("2022-10-24T02:00:44Z")},
				{Fingerprint: labels.EmptyLabels().Hash(), Labels: labels.EmptyLabels(), Start: timeParse("2022-10-19T10:50:44Z"), End: timeParse("2022-10-19T12:00:44Z")},
				{Fingerprint: labels.EmptyLabels().Hash(), Labels: labels.EmptyLabels(), Start: timeParse("2022-10-24T10:00:44Z"), End: timeParse("2022-10-24T18:00:44Z")},
				{Fingerprint: labels.EmptyLabels().Hash(), Labels: labels.EmptyLabels(), Start: timeParse("2022-10-23T14:00:44Z"), End: timeParse("2022-10-23T22:00:44Z")},
				{Fingerprint: labels.EmptyLabels().Hash(), Labels: labels.EmptyLabels(), Start: timeParse("2022-10-25T22:00:44Z"), End: timeParse("2022-10-26T10:55:44Z")},
				{Fingerprint: labels.EmptyLabels().Hash(), Labels: labels.EmptyLabels(), Start: timeParse("2022-10-20T14:00:44Z"), End: timeParse("2022-10-21T02:00:44Z")},
				{Fingerprint: labels.EmptyLabels().Hash(), Labels: labels.EmptyLabels(), Start: timeParse("2022-10-21T06:00:44Z"), End: timeParse("2022-10-21T20:00:44Z")},
				{Fingerprint: labels.EmptyLabels().Hash(), Labels: labels.EmptyLabels(), Start: timeParse("2022-10-21T20:00:44Z"), End: timeParse("2022-10-22T06:00:44Z")},
				{Fingerprint: labels.EmptyLabels().Hash(), Labels: labels.EmptyLabels(), Start: timeParse("2022-10-21T02:00:44Z"), End: timeParse("2022-10-21T06:00:44Z")},
				{Fingerprint: labels.EmptyLabels().Hash(), Labels: labels.EmptyLabels(), Start: timeParse("2022-10-23T22:00:44Z"), End: timeParse("2022-10-24T00:00:44Z")},
				{Fingerprint: labels.EmptyLabels().Hash(), Labels: labels.EmptyLabels(), Start: timeParse("2022-10-22T06:00:44Z"), End: timeParse("2022-10-22T12:00:44Z")},
			},
			out: promapi.MetricTimeRanges{
				{Fingerprint: labels.EmptyLabels().Hash(), Labels: labels.EmptyLabels(), Start: timeParse("2022-10-19T10:50:44Z"), End: timeParse("2022-10-26T10:56:43Z")},
			},
			step:      time.Minute,
			wasMerged: true,
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			for range 50 {
				out, wasMerged := promapi.MergeRanges(tc.in, tc.step)
				promapi.ExpandRangesEnd(out, tc.step)
				require.Equal(t, tc.wasMerged, wasMerged)
				slices.SortStableFunc(tc.in, promapi.CompareMetricTimeRanges)
				require.Equal(t, printRange(tc.out), printRange(out))
			}
		})
	}
}

func TestMetricTimeRangeOverlaps(t *testing.T) {
	type testCaseT struct {
		out  promapi.TimeRange
		desc string
		a    promapi.MetricTimeRange
		b    promapi.MetricTimeRange
		step time.Duration
		ok   bool
	}

	timeParse := func(s string) time.Time {
		v, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatal(err)
		}
		return v.UTC()
	}

	makeRange := func(ls labels.Labels, startOffset, endOffset time.Duration) promapi.MetricTimeRange {
		if startOffset >= endOffset {
			panic(fmt.Sprintf("startOffset must be < endOffset, got %s ~ %s", startOffset, endOffset))
		}
		ts := timeParse("2022-10-10T12:00:00Z")
		return promapi.MetricTimeRange{
			Fingerprint: ls.Hash(),
			Labels:      ls,
			Start:       ts.Add(startOffset),
			End:         ts.Add(endOffset),
		}
	}

	makeTime := func(startOffset, endOffset time.Duration) promapi.TimeRange {
		mtr := makeRange(labels.EmptyLabels(), startOffset, endOffset)
		return promapi.TimeRange{Start: mtr.Start, End: mtr.End}
	}

	testCases := []testCaseT{
		{
			desc: "0. different labels",
			a:    makeRange(labels.EmptyLabels(), 0, time.Hour),
			b:    makeRange(labels.FromStrings("key", "val"), 0, time.Hour),
			ok:   false,
		},
		{
			desc: "0. different labels",
			a:    makeRange(labels.EmptyLabels(), 0, time.Hour),
			b:    makeRange(labels.FromStrings("key", "val"), time.Hour*2, time.Hour*3),
			ok:   false,
		},
		{
			desc: "1. Equal",
			a:    makeRange(labels.EmptyLabels(), 0, time.Hour),
			b:    makeRange(labels.EmptyLabels(), 0, time.Hour),
			step: time.Minute,
			ok:   true,
			out:  makeTime(0, time.Hour),
		},
		{
			desc: "2. Overlap e1 and s2",
			a:    makeRange(labels.EmptyLabels(), 0, time.Hour),
			b:    makeRange(labels.EmptyLabels(), time.Second*30, time.Hour+time.Second*30),
			step: time.Minute,
			ok:   true,
			out:  makeTime(0, time.Hour+time.Second*30),
		},
		{
			desc: "3. Overlap e2 and s1",
			a:    makeRange(labels.EmptyLabels(), 0, time.Hour),
			b:    makeRange(labels.EmptyLabels(), time.Second*-30, time.Hour+time.Second*-30),
			step: time.Minute,
			ok:   true,
			out:  makeTime(time.Second*-30, time.Hour),
		},
		{
			desc: "4. s2 continues e1",
			a:    makeRange(labels.EmptyLabels(), 0, time.Hour),
			b:    makeRange(labels.EmptyLabels(), time.Hour, time.Hour*2),
			step: time.Minute,
			ok:   true,
			out:  makeTime(0, time.Hour*2),
		},
		{
			desc: "4. s2 continues e1",
			a:    makeRange(labels.EmptyLabels(), 0, time.Hour),
			b:    makeRange(labels.EmptyLabels(), time.Hour+time.Minute, time.Hour*2),
			step: time.Minute,
			ok:   true,
			out:  makeTime(0, time.Hour*2),
		},
		{
			desc: "5. s1 continues e2",
			a:    makeRange(labels.EmptyLabels(), time.Hour, time.Hour*2),
			b:    makeRange(labels.EmptyLabels(), 0, time.Hour),
			step: time.Minute,
			ok:   true,
			out:  makeTime(0, time.Hour*2),
		},
		{
			desc: "5. s1 continues e2",
			a:    makeRange(labels.EmptyLabels(), time.Hour+time.Minute, time.Hour*2),
			b:    makeRange(labels.EmptyLabels(), 0, time.Hour),
			step: time.Minute,
			ok:   true,
			out:  makeTime(0, time.Hour*2),
		},
		{
			desc: "6. Second range fully included in first range",
			a:    makeRange(labels.EmptyLabels(), 0, time.Hour*4),
			b:    makeRange(labels.EmptyLabels(), time.Hour, time.Hour*2),
			step: time.Minute,
			ok:   true,
			out:  makeTime(0, time.Hour*4),
		},
		{
			desc: "7. Second range included in first range (start aligned)",
			a:    makeRange(labels.EmptyLabels(), 0, time.Hour*2),
			b:    makeRange(labels.EmptyLabels(), 0, time.Hour),
			step: time.Minute,
			ok:   true,
			out:  makeTime(0, time.Hour*2),
		},
		{
			desc: "7. Second range included in first range (start aligned)",
			a:    makeRange(labels.EmptyLabels(), 0, time.Hour*2),
			b:    makeRange(labels.EmptyLabels(), time.Second*30, time.Hour),
			step: time.Minute,
			ok:   true,
			out:  makeTime(0, time.Hour*2),
		},
		{
			desc: "7. Second range included in first range (start aligned)",
			a:    makeRange(labels.EmptyLabels(), 0, time.Hour*2),
			b:    makeRange(labels.EmptyLabels(), 0, time.Hour*2-time.Second*30),
			step: time.Minute,
			ok:   true,
			out:  makeTime(0, time.Hour*2),
		},
		{
			desc: "8. Second range included in first range (end aligned)",
			a:    makeRange(labels.EmptyLabels(), 0, time.Hour*2),
			b:    makeRange(labels.EmptyLabels(), time.Hour, time.Hour*2),
			step: time.Minute,
			ok:   true,
			out:  makeTime(0, time.Hour*2),
		},
		{
			desc: "8. Second range included in first range (end aligned)",
			a:    makeRange(labels.EmptyLabels(), 0, time.Hour*2),
			b:    makeRange(labels.EmptyLabels(), time.Hour, time.Hour*2-time.Second*30),
			step: time.Minute,
			ok:   true,
			out:  makeTime(0, time.Hour*2),
		},
		{
			desc: "8. Second range included in first range (end aligned)",
			a:    makeRange(labels.EmptyLabels(), 0, time.Hour*2),
			b:    makeRange(labels.EmptyLabels(), time.Hour, time.Hour*2+time.Second*30),
			step: time.Minute,
			ok:   true,
			out:  makeTime(0, time.Hour*2+time.Second*30),
		},
		{
			desc: "9. First range fully included in second range",
			a:    makeRange(labels.EmptyLabels(), time.Hour, time.Hour*2),
			b:    makeRange(labels.EmptyLabels(), 0, time.Hour*4),
			step: time.Minute,
			ok:   true,
			out:  makeTime(0, time.Hour*4),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			out, ok := promapi.Overlaps(tc.a, tc.b, tc.step)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.out, out)
		})
	}
}

func TestMetricTimeRangesString(t *testing.T) {
	timeParse := func(s string) time.Time {
		v, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatal(err)
		}
		return v.UTC()
	}

	type testCaseT struct {
		output string
		ranges promapi.MetricTimeRanges
	}

	testCases := []testCaseT{
		{
			ranges: promapi.MetricTimeRanges{},
			output: "",
		},
		{
			ranges: promapi.MetricTimeRanges{
				{
					Labels: labels.FromStrings("instance", "foo"),
					Start:  timeParse("2022-06-14T00:00:00Z"),
					End:    timeParse("2022-06-14T01:00:00Z"),
				},
			},
			output: `{instance="foo"} 2022-06-14T00:00:00Z > 2022-06-14T01:00:00Z`,
		},
		{
			ranges: promapi.MetricTimeRanges{
				{
					Labels: labels.FromStrings("instance", "foo"),
					Start:  timeParse("2022-06-14T00:00:00Z"),
					End:    timeParse("2022-06-14T01:00:00Z"),
				},
				{
					Labels: labels.FromStrings("instance", "bar"),
					Start:  timeParse("2022-06-14T02:00:00Z"),
					End:    timeParse("2022-06-14T03:00:00Z"),
				},
			},
			output: `{instance="foo"} 2022-06-14T00:00:00Z > 2022-06-14T01:00:00Z ** {instance="bar"} 2022-06-14T02:00:00Z > 2022-06-14T03:00:00Z`,
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			result := tc.ranges.String()
			require.Equal(t, tc.output, result)
		})
	}
}

func TestMergeRangesWithoutGaps(t *testing.T) {
	timeParse := func(s string) time.Time {
		v, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatal(err)
		}
		return v.UTC()
	}

	fpA := labels.FromStrings("job", "a").Hash()
	fpB := labels.FromStrings("job", "b").Hash()
	labelsA := labels.FromStrings("job", "a")
	labelsB := labels.FromStrings("job", "b")

	type testCaseT struct {
		desc   string
		ranges promapi.MetricTimeRanges
		gaps   []promapi.TimeRange
		out    promapi.MetricTimeRanges
	}

	testCases := []testCaseT{
		{
			// No gaps at all, ranges returned as-is.
			desc: "no gaps returns ranges unchanged",
			ranges: promapi.MetricTimeRanges{
				{Fingerprint: fpA, Labels: labelsA, Start: timeParse("2024-01-01T00:00:00Z"), End: timeParse("2024-01-01T00:07:00Z")},
				{Fingerprint: fpA, Labels: labelsA, Start: timeParse("2024-01-01T00:10:00Z"), End: timeParse("2024-01-01T00:17:00Z")},
			},
			gaps: nil,
			out: promapi.MetricTimeRanges{
				{Fingerprint: fpA, Labels: labelsA, Start: timeParse("2024-01-01T00:00:00Z"), End: timeParse("2024-01-01T00:07:00Z")},
				{Fingerprint: fpA, Labels: labelsA, Start: timeParse("2024-01-01T00:10:00Z"), End: timeParse("2024-01-01T00:17:00Z")},
			},
		},
		{
			// Single range, nothing to merge.
			desc: "single range returns as-is",
			ranges: promapi.MetricTimeRanges{
				{Fingerprint: fpA, Labels: labelsA, Start: timeParse("2024-01-01T00:00:00Z"), End: timeParse("2024-01-01T00:07:00Z")},
			},
			gaps: []promapi.TimeRange{
				{Start: timeParse("2024-01-01T00:07:00Z"), End: timeParse("2024-01-01T00:10:00Z")},
			},
			out: promapi.MetricTimeRanges{
				{Fingerprint: fpA, Labels: labelsA, Start: timeParse("2024-01-01T00:00:00Z"), End: timeParse("2024-01-01T00:07:00Z")},
			},
		},
		{
			// Two ranges with no gap between them merge.
			desc: "ranges without gap between them merge",
			ranges: promapi.MetricTimeRanges{
				{Fingerprint: fpA, Labels: labelsA, Start: timeParse("2024-01-01T00:00:00Z"), End: timeParse("2024-01-01T00:07:00Z")},
				{Fingerprint: fpA, Labels: labelsA, Start: timeParse("2024-01-01T00:10:00Z"), End: timeParse("2024-01-01T00:17:00Z")},
			},
			gaps: []promapi.TimeRange{
				{Start: timeParse("2024-01-01T00:20:00Z"), End: timeParse("2024-01-01T00:25:00Z")},
			},
			out: promapi.MetricTimeRanges{
				{Fingerprint: fpA, Labels: labelsA, Start: timeParse("2024-01-01T00:00:00Z"), End: timeParse("2024-01-01T00:17:00Z")},
			},
		},
		{
			// Two ranges with a gap between them stay separate.
			desc: "ranges with gap between them stay separate",
			ranges: promapi.MetricTimeRanges{
				{Fingerprint: fpA, Labels: labelsA, Start: timeParse("2024-01-01T00:00:00Z"), End: timeParse("2024-01-01T00:07:00Z")},
				{Fingerprint: fpA, Labels: labelsA, Start: timeParse("2024-01-01T00:10:00Z"), End: timeParse("2024-01-01T00:17:00Z")},
			},
			gaps: []promapi.TimeRange{
				{Start: timeParse("2024-01-01T00:07:00Z"), End: timeParse("2024-01-01T00:10:00Z")},
			},
			out: promapi.MetricTimeRanges{
				{Fingerprint: fpA, Labels: labelsA, Start: timeParse("2024-01-01T00:00:00Z"), End: timeParse("2024-01-01T00:07:00Z")},
				{Fingerprint: fpA, Labels: labelsA, Start: timeParse("2024-01-01T00:10:00Z"), End: timeParse("2024-01-01T00:17:00Z")},
			},
		},
		{
			// Different fingerprints are never merged.
			desc: "different fingerprints stay separate even without gap",
			ranges: promapi.MetricTimeRanges{
				{Fingerprint: fpA, Labels: labelsA, Start: timeParse("2024-01-01T00:00:00Z"), End: timeParse("2024-01-01T00:07:00Z")},
				{Fingerprint: fpB, Labels: labelsB, Start: timeParse("2024-01-01T00:10:00Z"), End: timeParse("2024-01-01T00:17:00Z")},
			},
			gaps: []promapi.TimeRange{
				{Start: timeParse("2024-01-01T00:20:00Z"), End: timeParse("2024-01-01T00:25:00Z")},
			},
			out: promapi.MetricTimeRanges{
				{Fingerprint: fpA, Labels: labelsA, Start: timeParse("2024-01-01T00:00:00Z"), End: timeParse("2024-01-01T00:07:00Z")},
				{Fingerprint: fpB, Labels: labelsB, Start: timeParse("2024-01-01T00:10:00Z"), End: timeParse("2024-01-01T00:17:00Z")},
			},
		},
		{
			// Second range is fully inside first range, no extension needed.
			desc: "overlapping range does not extend end",
			ranges: promapi.MetricTimeRanges{
				{Fingerprint: fpA, Labels: labelsA, Start: timeParse("2024-01-01T00:00:00Z"), End: timeParse("2024-01-01T00:20:00Z")},
				{Fingerprint: fpA, Labels: labelsA, Start: timeParse("2024-01-01T00:05:00Z"), End: timeParse("2024-01-01T00:15:00Z")},
			},
			gaps: []promapi.TimeRange{
				{Start: timeParse("2024-01-01T00:25:00Z"), End: timeParse("2024-01-01T00:30:00Z")},
			},
			out: promapi.MetricTimeRanges{
				{Fingerprint: fpA, Labels: labelsA, Start: timeParse("2024-01-01T00:00:00Z"), End: timeParse("2024-01-01T00:20:00Z")},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			result := promapi.MergeRangesWithoutGaps(tc.ranges, tc.gaps)
			require.Equal(t, tc.out, result)
		})
	}
}

func TestHasGapBetween(t *testing.T) {
	timeParse := func(s string) time.Time {
		v, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatal(err)
		}
		return v.UTC()
	}

	type testCaseT struct {
		desc string
		a    time.Time
		b    time.Time
		gaps []promapi.TimeRange
		out  bool
	}

	testCases := []testCaseT{
		{
			// No gaps at all.
			desc: "no gaps",
			a:    timeParse("2024-01-01T00:00:00Z"),
			b:    timeParse("2024-01-01T01:00:00Z"),
			gaps: nil,
			out:  false,
		},
		{
			// Gap fully inside the interval.
			desc: "gap inside interval",
			a:    timeParse("2024-01-01T00:00:00Z"),
			b:    timeParse("2024-01-01T01:00:00Z"),
			gaps: []promapi.TimeRange{
				{Start: timeParse("2024-01-01T00:20:00Z"), End: timeParse("2024-01-01T00:40:00Z")},
			},
			out: true,
		},
		{
			// Gap ends before the interval starts.
			desc: "gap before interval",
			a:    timeParse("2024-01-01T01:00:00Z"),
			b:    timeParse("2024-01-01T02:00:00Z"),
			gaps: []promapi.TimeRange{
				{Start: timeParse("2024-01-01T00:00:00Z"), End: timeParse("2024-01-01T00:30:00Z")},
			},
			out: false,
		},
		{
			// Gap starts after the interval ends.
			desc: "gap after interval",
			a:    timeParse("2024-01-01T00:00:00Z"),
			b:    timeParse("2024-01-01T01:00:00Z"),
			gaps: []promapi.TimeRange{
				{Start: timeParse("2024-01-01T02:00:00Z"), End: timeParse("2024-01-01T03:00:00Z")},
			},
			out: false,
		},
		{
			// Gap partially overlaps the interval.
			desc: "gap partially overlaps interval",
			a:    timeParse("2024-01-01T00:00:00Z"),
			b:    timeParse("2024-01-01T01:00:00Z"),
			gaps: []promapi.TimeRange{
				{Start: timeParse("2024-01-01T00:30:00Z"), End: timeParse("2024-01-01T01:30:00Z")},
			},
			out: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			result := promapi.HasGapBetween(tc.a, tc.b, tc.gaps)
			require.Equal(t, tc.out, result)
		})
	}
}
