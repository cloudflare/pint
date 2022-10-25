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
	"github.com/stretchr/testify/require"
)

func TestAppendSamplesToRanges(t *testing.T) {
	type testCaseT struct {
		in      []promapi.MetricTimeRange
		samples []model.SampleStream
		step    time.Duration
		out     []promapi.MetricTimeRange
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
					Fingerprint: model.LabelSet{"instance": "1"}.Fingerprint(),
					Labels:      model.LabelSet{"instance": "1"},
					Start:       timeParse("2022-06-14T00:00:00Z"),
					End:         timeParse("2022-06-14T03:00:00Z"),
				},
				{
					Fingerprint: model.LabelSet{"instance": "2"}.Fingerprint(),
					Labels:      model.LabelSet{"instance": "2"},
					Start:       timeParse("2022-06-14T00:00:00Z"),
					End:         timeParse("2022-06-14T03:00:00Z"),
				},
				{
					Fingerprint: model.LabelSet{"instance": "3"}.Fingerprint(),
					Labels:      model.LabelSet{"instance": "3"},
					Start:       timeParse("2022-06-14T00:00:00Z"),
					End:         timeParse("2022-06-14T03:00:00Z"),
				},
			},
		},
		{
			in: []promapi.MetricTimeRange{
				{
					Fingerprint: model.LabelSet{"instance": "1"}.Fingerprint(),
					Labels:      model.LabelSet{"instance": "1"},
					Start:       timeParse("2022-06-14T00:00:00Z"),
					End:         timeParse("2022-06-14T03:00:00Z"),
				},
				{
					Fingerprint: model.LabelSet{"instance": "3"}.Fingerprint(),
					Labels:      model.LabelSet{"instance": "3"},
					Start:       timeParse("2022-06-14T00:00:00Z"),
					End:         timeParse("2022-06-14T03:00:00Z"),
				},
				{
					Fingerprint: model.LabelSet{"instance": "2"}.Fingerprint(),
					Labels:      model.LabelSet{"instance": "2"},
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
					Fingerprint: model.LabelSet{"instance": "1"}.Fingerprint(),
					Labels:      model.LabelSet{"instance": "1"},
					Start:       timeParse("2022-06-13T10:00:00Z"),
					End:         timeParse("2022-06-13T13:00:00Z"),
				},
				{
					Fingerprint: model.LabelSet{"instance": "1"}.Fingerprint(),
					Labels:      model.LabelSet{"instance": "1"},
					Start:       timeParse("2022-06-13T23:00:00Z"),
					End:         timeParse("2022-06-14T04:00:00Z"),
				},
				{
					Fingerprint: model.LabelSet{"instance": "2"}.Fingerprint(),
					Labels:      model.LabelSet{"instance": "2"},
					Start:       timeParse("2022-06-14T00:00:00Z"),
					End:         timeParse("2022-06-14T03:00:00Z"),
				},
				{
					Fingerprint: model.LabelSet{"instance": "2"}.Fingerprint(),
					Labels:      model.LabelSet{"instance": "2"},
					Start:       timeParse("2022-06-15T10:00:00Z"),
					End:         timeParse("2022-06-15T13:00:00Z"),
				},
				{
					Fingerprint: model.LabelSet{"instance": "3"}.Fingerprint(),
					Labels:      model.LabelSet{"instance": "3"},
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
					Fingerprint: model.LabelSet{"instance": "1"}.Fingerprint(),
					Labels:      model.LabelSet{"instance": "1"},
					Start:       timeParse("2022-06-14T00:00:00Z"),
					End:         timeParse("2022-06-14T08:00:00Z"),
				},
			},
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			out := promapi.AppendSamplesToRanges(tc.in, tc.samples, tc.step)
			sort.Stable(out)
			require.Equal(t, printRange(tc.out), printRange(out))
		})
	}
}
