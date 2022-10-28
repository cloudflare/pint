package promapi_test

import (
	"fmt"
	"sort"
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
		step    time.Duration
		out     promapi.MetricTimeRanges
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
			buf.WriteString(fmt.Sprintf("%s %s - %s\n", r.Labels, r.Start.UTC().Format(time.RFC3339), r.End.UTC().Format(time.RFC3339)))
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
					End:         timeParse("2022-06-14T03:00:00Z"),
				},
				{
					Fingerprint: labels.FromStrings("instance", "2").Hash(),
					Labels:      labels.FromStrings("instance", "2"),
					Start:       timeParse("2022-06-14T00:00:00Z"),
					End:         timeParse("2022-06-14T03:00:00Z"),
				},
				{
					Fingerprint: labels.FromStrings("instance", "3").Hash(),
					Labels:      labels.FromStrings("instance", "3"),
					Start:       timeParse("2022-06-14T00:00:00Z"),
					End:         timeParse("2022-06-14T03:00:00Z"),
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
					End:         timeParse("2022-06-14T03:00:00Z"),
				},
				{
					Fingerprint: labels.FromStrings("job", "bar").Hash(),
					Labels:      labels.FromStrings("job", "bar"),
					Start:       timeParse("2022-06-14T00:00:00Z"),
					End:         timeParse("2022-06-14T03:00:00Z"),
				},
				{
					Fingerprint: labels.FromStrings("instance", "1").Hash(),
					Labels:      labels.FromStrings("instance", "1"),
					Start:       timeParse("2022-06-14T00:00:00Z"),
					End:         timeParse("2022-06-14T03:00:00Z"),
				},
				{
					Fingerprint: labels.FromStrings("instance", "1", "job", "foo").Hash(),
					Labels:      labels.FromStrings("instance", "1", "job", "foo"),
					Start:       timeParse("2022-06-14T00:00:00Z"),
					End:         timeParse("2022-06-14T03:00:00Z"),
				},
			},
		},
		{
			in: []promapi.MetricTimeRange{
				{
					Fingerprint: labels.FromStrings("instance", "1").Hash(),
					Labels:      labels.FromStrings("instance", "1"),
					Start:       timeParse("2022-06-14T00:00:00Z"),
					End:         timeParse("2022-06-14T03:00:00Z"),
				},
				{
					Fingerprint: labels.FromStrings("instance", "3").Hash(),
					Labels:      labels.FromStrings("instance", "3"),
					Start:       timeParse("2022-06-14T00:00:00Z"),
					End:         timeParse("2022-06-14T03:00:00Z"),
				},
				{
					Fingerprint: labels.FromStrings("instance", "2").Hash(),
					Labels:      labels.FromStrings("instance", "2"),
					Start:       timeParse("2022-06-14T00:00:00Z"),
					End:         timeParse("2022-06-14T03:00:00Z"),
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
					End:         timeParse("2022-06-13T13:00:00Z"),
				},
				{
					Fingerprint: labels.FromStrings("instance", "1").Hash(),
					Labels:      labels.FromStrings("instance", "1"),
					Start:       timeParse("2022-06-13T23:00:00Z"),
					End:         timeParse("2022-06-14T04:00:00Z"),
				},
				{
					Fingerprint: labels.FromStrings("instance", "2").Hash(),
					Labels:      labels.FromStrings("instance", "2"),
					Start:       timeParse("2022-06-14T00:00:00Z"),
					End:         timeParse("2022-06-14T03:00:00Z"),
				},
				{
					Fingerprint: labels.FromStrings("instance", "2").Hash(),
					Labels:      labels.FromStrings("instance", "2"),
					Start:       timeParse("2022-06-15T10:00:00Z"),
					End:         timeParse("2022-06-15T13:00:00Z"),
				},
				{
					Fingerprint: labels.FromStrings("instance", "3").Hash(),
					Labels:      labels.FromStrings("instance", "3"),
					Start:       timeParse("2022-06-14T00:00:00Z"),
					End:         timeParse("2022-06-14T03:00:00Z"),
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
					End:         timeParse("2022-06-14T08:00:00Z"),
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
					End:         timeParse("2022-10-27T09:21:59Z"),
				},
				{
					Fingerprint: labels.FromStrings("instance", "1").Hash(),
					Labels:      labels.FromStrings("instance", "1"),
					Start:       timeParse("2022-10-27T10:14:59Z"),
					End:         timeParse("2022-10-27T10:21:59Z"),
				},
				{
					Fingerprint: labels.FromStrings("instance", "1").Hash(),
					Labels:      labels.FromStrings("instance", "1"),
					Start:       timeParse("2022-10-27T11:14:59Z"),
					End:         timeParse("2022-10-27T11:16:59Z"),
				},
				{
					Fingerprint: labels.FromStrings("instance", "1").Hash(),
					Labels:      labels.FromStrings("instance", "1"),
					Start:       timeParse("2022-10-27T12:14:59Z"),
					End:         timeParse("2022-10-27T12:31:59Z"),
				},
				{
					Fingerprint: labels.FromStrings("instance", "1").Hash(),
					Labels:      labels.FromStrings("instance", "1"),
					Start:       timeParse("2022-10-27T13:14:59Z"),
					End:         timeParse("2022-10-27T13:51:59Z"),
				},
				{
					Fingerprint: labels.FromStrings("instance", "1").Hash(),
					Labels:      labels.FromStrings("instance", "1"),
					Start:       timeParse("2022-10-27T14:14:59Z"),
					End:         timeParse("2022-10-27T14:51:59Z"),
				},
				{
					Fingerprint: labels.FromStrings("instance", "1").Hash(),
					Labels:      labels.FromStrings("instance", "1"),
					Start:       timeParse("2022-10-27T23:14:59Z"),
					End:         timeParse("2022-10-28T01:15:59Z"),
				},
			},
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			for _, s := range tc.samples {
				lset := promapi.MetricToLabels(s.Metric)
				tc.in = promapi.AppendSampleToRanges(tc.in, lset, s.Values, tc.step)
			}
			tc.in = promapi.MergeRanges(tc.in, tc.step)
			sort.Stable(tc.in)
			require.Equal(t, printRange(tc.out), printRange(tc.in))
		})
	}
}

func TestMergeRanges(t *testing.T) {
	type testCaseT struct {
		in   promapi.MetricTimeRanges
		out  promapi.MetricTimeRanges
		step time.Duration
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
			buf.WriteString(fmt.Sprintf("%s %s - %s\n", r.Labels, r.Start.UTC().Format(time.RFC3339), r.End.UTC().Format(time.RFC3339)))
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
				{Fingerprint: labels.EmptyLabels().Hash(), Labels: labels.EmptyLabels(), Start: timeParse("2022-10-19T10:50:44Z"), End: timeParse("2022-10-26T10:55:44Z")},
			},
			step: time.Minute,
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			out := promapi.MergeRanges(tc.in, tc.step)
			sort.Stable(out)
			require.Equal(t, printRange(tc.out), printRange(out))
		})
	}
}
