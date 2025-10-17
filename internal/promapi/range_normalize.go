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
	// 0. Different labels
	if a.Fingerprint != b.Fingerprint {
		return c, false
	}

	// 1. Equal (within step)
	//    [s1 e1]
	//    [s2 e2]
	if a.Start.Sub(b.Start).Abs() <= step && a.End.Sub(b.End).Abs() <= step {
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

	// 2. Overlap e1 and s2
	//    [s1 e1]
	//      [s2 es2]
	if a.Start.Before(b.Start) && a.End.After(b.Start) && a.End.Before(b.End) {
		c.Start = a.Start
		c.End = b.End
		return c, true
	}

	// 3. Overlap e2 and s1
	//      [s1 e2]
	//    [s2 e2]
	if a.Start.After(b.Start) && a.Start.Before(b.End) && a.End.After(b.End) {
		c.Start = b.Start
		c.End = a.End
		return c, true
	}

	// 4. s2 continues e1
	//    [s1 e1]
	//           [s2 e2]
	if a.Start.Before(b.Start) && a.End.Before(b.End) && a.End.Sub(b.Start).Abs() <= step {
		c.Start = a.Start
		c.End = b.End
		return c, true
	}

	// 5. s1 continues e2
	//           [s1 e1]
	//    [s2 e2]
	if a.Start.After(b.Start) && a.End.After(b.End) && a.Start.Sub(b.End).Abs() <= step {
		c.Start = b.Start
		c.End = a.End
		return c, true
	}

	// 6. Second range fully included in first range
	//    [s1     e1]
	//      [s2 e2]
	if a.Start.Before(b.Start) && a.End.After(b.End) {
		c.Start = a.Start
		c.End = a.End
		return c, true
	}

	// 7. Second range included in first range (start aligned)
	//    [s1   e1]
	//    [s2 e2]
	if a.Start.Sub(b.Start).Abs() <= step && a.End.After(b.End) {
		if a.Start.Before(b.Start) {
			c.Start = a.Start
		} else {
			c.Start = b.Start
		}
		c.End = a.End
		return c, true
	}

	// 8. Second range included in first range (end aligned)
	//    [s1   e1]
	//      [s2 e2]
	if a.Start.Before(b.Start) && a.End.Sub(b.End).Abs() <= step {
		c.Start = a.Start
		if a.End.After(b.End) {
			c.End = a.End
		} else {
			c.End = b.End
		}
		return c, true
	}

	// 9. First range fully included in second range
	//      [s1 e1]
	//    [s2     e2]
	if a.Start.After(b.Start) && a.End.Before(b.End) {
		c.Start = b.Start
		c.End = b.End
		return c, true
	}

	return c, false
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
