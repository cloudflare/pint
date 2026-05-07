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

// Overlaps returns the union of two ranges if they overlap or are within one step of each other.
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

// CompareMetricTimeRanges sorts by labels first, then by start time.
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

// covers returns true if any range contains the given timestamp.
func (str SeriesTimeRanges) covers(ts time.Time) bool {
	for _, r := range str.Ranges {
		if !r.Start.After(ts) && !r.End.Before(ts) {
			return true
		}
	}
	return false
}

// FindGaps records time periods where str has no data but baseline does.
// If baseline has data at a timestamp but str doesn't, the metric was genuinely absent.
// If baseline also has no data, the source was down and we can't tell.
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

// MergeRanges merges overlapping or adjacent (within one step) ranges.
// Sorts the source slice as a side effect.
func MergeRanges(source MetricTimeRanges, step time.Duration) (MetricTimeRanges, bool) {
	slices.SortStableFunc(source, CompareMetricTimeRanges)

	var (
		ok, hadMerged bool
		tr            TimeRange
	)

	merged := make(MetricTimeRanges, 0, len(source))
L:
	for i := range source {
		for j, m := range slices.Backward(merged) {
			if source[i].Fingerprint != m.Fingerprint {
				continue
			}
			if tr, ok = Overlaps(m, source[i], step); ok {
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

// MergeRangesWithoutGaps merges consecutive same-fingerprint ranges unless
// a gap (from FindGaps) separates them. Ranges separated only by periods
// where the source was down get merged because the condition might have been true.
func MergeRangesWithoutGaps(
	ranges MetricTimeRanges,
	gaps []TimeRange,
) MetricTimeRanges {
	if len(gaps) == 0 || len(ranges) < 2 {
		return ranges
	}

	merged := make(MetricTimeRanges, 0, len(ranges))
	merged = append(merged, ranges[0])
	for i := 1; i < len(ranges); i++ {
		last := &merged[len(merged)-1]
		if last.Fingerprint != ranges[i].Fingerprint {
			merged = append(merged, ranges[i])
			continue
		}
		if HasGapBetween(last.End, ranges[i].Start, gaps) {
			merged = append(merged, ranges[i])
			continue
		}
		if ranges[i].End.After(last.End) {
			last.End = ranges[i].End
		}
	}
	return merged
}

// HasGapBetween returns true if any gap from FindGaps overlaps the interval [a, b].
func HasGapBetween(a, b time.Time, gaps []TimeRange) bool {
	for _, g := range gaps {
		if !g.End.Before(a) && !g.Start.After(b) {
			return true
		}
	}
	return false
}

// ExpandRangesEnd extends each range's end by step-1s to cover the full sample interval.
func ExpandRangesEnd(src MetricTimeRanges, step time.Duration) {
	for i := range src {
		src[i].End = src[i].End.Add(step - time.Second)
	}
}

// AppendSampleToRanges adds sample timestamps to existing ranges or creates new ones.
// Samples within one step of an existing range extend it; otherwise a new range is created.
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

// MetricToLabels converts a Prometheus model.Metric to a labels.Labels.
func MetricToLabels(m model.Metric) labels.Labels {
	lset := make([]string, 0, len(m)*2)
	for k, v := range m {
		lset = append(lset, string(k), string(v))
	}
	return labels.FromStrings(lset...)
}
