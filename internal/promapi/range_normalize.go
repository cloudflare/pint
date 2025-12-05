package promapi

import (
	"slices"
	"strings"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
)

type TimeRange struct {
	Start time.Time
	End   time.Time
}

type MetricTimeRange struct {
	Start       time.Time
	End         time.Time
	Labels      labels.Labels
	Fingerprint uint64
}

func Overlaps(a, b MetricTimeRange, step time.Duration) (c TimeRange, ok bool) {
	// Different labels cannot overlap.
	if a.Fingerprint != b.Fingerprint {
		return c, false
	}

	// Ranges can be merged if they overlap or the gap between them is <= step.
	// If a ends before b starts, gap = b.Start - a.End.
	// If b ends before a starts, gap = a.Start - b.End.
	// Otherwise they overlap (gap <= 0).
	if a.End.Before(b.Start) && b.Start.Sub(a.End) > step {
		return c, false
	}
	if b.End.Before(a.Start) && a.Start.Sub(b.End) > step {
		return c, false
	}

	// Ranges can be merged - return their union.
	if a.Start.Before(b.Start) {
		c.Start = a.Start
	} else {
		c.Start = b.Start
	}
	if a.End.After(b.End) {
		c.End = a.End
	} else {
		c.End = b.End
	}

	return c, true
}

type MetricTimeRanges []MetricTimeRange

func (mtr MetricTimeRanges) String() string {
	sl := make([]string, 0, len(mtr))
	for _, tr := range mtr {
		sl = append(sl, tr.Labels.String()+" "+tr.Start.UTC().Format(time.RFC3339)+" > "+tr.End.UTC().Format(time.RFC3339))
	}
	return strings.Join(sl, " ** ")
}

func CompareMetricTimeRanges(a, b MetricTimeRange) int {
	if a.Fingerprint != b.Fingerprint {
		return labels.Compare(a.Labels, b.Labels)
	}
	return a.Start.Compare(b.Start)
}

type SeriesTimeRanges struct {
	From   time.Time
	Until  time.Time
	Ranges MetricTimeRanges
	Gaps   []TimeRange
	Step   time.Duration
}

func (str SeriesTimeRanges) covers(ts time.Time) bool {
	for _, r := range str.Ranges {
		if !r.Start.After(ts) && !r.End.Before(ts) {
			return true
		}
	}
	return false
}

func (str *SeriesTimeRanges) FindGaps(baseline SeriesTimeRanges, from, until time.Time) {
	for !from.After(until) {
		if str.covers(from) || !baseline.covers(from) {
			from = from.Add(str.Step)
			continue
		}

		var found bool
		for i := range str.Gaps {
			if !from.Before(str.Gaps[i].Start) &&
				!from.After(str.Gaps[i].End.Add(str.Step)) {
				str.Gaps[i].End = from.Add(str.Step)
				found = true
				break
			}
		}
		if !found {
			str.Gaps = append(str.Gaps, TimeRange{Start: from, End: from.Add(str.Step)})
		}

		from = from.Add(str.Step)
	}
}

// merge [t1:t2] [t2:t3] together.
// This will sort the source slice.
func MergeRanges(source MetricTimeRanges, step time.Duration) (MetricTimeRanges, bool) {
	slices.SortStableFunc(source, CompareMetricTimeRanges)

	var (
		ok, hadMerged bool
		tr            TimeRange
	)

	merged := make(MetricTimeRanges, 0, len(source))
L:
	for i := range source {
		for j := len(merged) - 1; j >= 0; j-- {
			if source[i].Fingerprint != merged[j].Fingerprint {
				continue
			}
			if tr, ok = Overlaps(merged[j], source[i], step); ok {
				merged[j].Start = tr.Start
				merged[j].End = tr.End
				hadMerged = true
				continue L
			}
		}
		merged = append(merged, source[i])
	}

	return merged, hadMerged
}

func ExpandRangesEnd(src MetricTimeRanges, step time.Duration) {
	for i := range src {
		src[i].End = src[i].End.Add(step - time.Second)
	}
}

func AppendSampleToRanges(dst MetricTimeRanges, ls labels.Labels, vals []model.SamplePair, step time.Duration) MetricTimeRanges {
	fp := ls.Hash()

	var ts time.Time
	var found bool
	for _, v := range vals {
		ts = v.Timestamp.Time()

		found = false
		for i := range dst {
			if dst[i].Fingerprint != fp {
				continue
			}

			if !ts.Before(dst[i].Start.Add(step*-1)) && !ts.After(dst[i].Start) {
				dst[i].Start = ts
				found = true
				break
			}

			if !ts.Before(dst[i].Start) &&
				!ts.After(dst[i].End.Add(step)) {
				dst[i].End = ts
				found = true
				break
			}
		}
		if !found {
			dst = append(dst, MetricTimeRange{
				Fingerprint: fp,
				Labels:      ls,
				Start:       ts,
				End:         ts,
			})
		}
	}

	return dst
}

func MetricToLabels(m model.Metric) labels.Labels {
	lset := make([]string, 0, len(m)*2)
	for k, v := range m {
		lset = append(lset, string(k), string(v))
	}
	return labels.FromStrings(lset...)
}
