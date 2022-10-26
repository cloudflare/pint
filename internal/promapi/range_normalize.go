package promapi

import (
	"sort"
	"time"

	"github.com/prometheus/common/model"
	"golang.org/x/exp/slices"
)

type TimeRange struct {
	Start time.Time
	End   time.Time
}

type MetricTimeRange struct {
	Fingerprint model.Fingerprint
	Labels      model.LabelSet
	Start       time.Time
	End         time.Time
}

type MetricTimeRanges []MetricTimeRange

func (mtr MetricTimeRanges) Len() int {
	return len(mtr)
}

func (mtr MetricTimeRanges) Swap(i, j int) {
	mtr[i], mtr[j] = mtr[j], mtr[i]
}

func (mtr MetricTimeRanges) Less(i, j int) bool {
	if mtr[i].Fingerprint != mtr[j].Fingerprint {
		return mtr[i].Labels.Before(mtr[j].Labels)
	}
	return mtr[i].Start.Before(mtr[j].Start)
}

type SeriesTimeRanges struct {
	From   time.Time
	Until  time.Time
	Step   time.Duration
	Ranges MetricTimeRanges
	Gaps   []TimeRange
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
	for !from.After(str.Until) {
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

// merge [t1:t2] [t2:t3] together
func MergeRanges(dst MetricTimeRanges) MetricTimeRanges {
	sort.Stable(dst)

	var toPurge []int
	for i := range dst {
		for j := range dst {
			if i == j {
				continue
			}
			if dst[i].Fingerprint != dst[j].Fingerprint {
				continue
			}
			if slices.Contains(toPurge, j) {
				continue
			}
			if dst[i].Start.Before(dst[j].Start) && !dst[i].End.Before(dst[j].Start) && !dst[i].End.After(dst[j].End) {
				dst[i].End = dst[j].End
				toPurge = append(toPurge, j)
			}
		}
	}

	merged := make(MetricTimeRanges, 0, len(dst)-len(toPurge))
	for i, tr := range dst {
		for _, j := range toPurge {
			if i == j {
				goto NEXT
			}
		}
		merged = append(merged, tr)
	NEXT:
	}

	return merged
}

func AppendSampleToRanges(dst MetricTimeRanges, s model.SampleStream, step time.Duration) MetricTimeRanges {
	var ts time.Time
	var fp model.Fingerprint
	for _, v := range s.Values {
		ts = v.Timestamp.Time()
		ls := model.LabelSet(s.Metric)
		fp = ls.Fingerprint()

		var found bool
		for i := range dst {
			if dst[i].Fingerprint == fp &&
				!ts.Before(dst[i].Start) &&
				!ts.After(dst[i].End.Add(step)) {
				dst[i].End = ts.Add(step)
				found = true
				break
			}
		}
		if !found {
			dst = append(dst, MetricTimeRange{
				Fingerprint: fp,
				Labels:      ls,
				Start:       ts,
				End:         ts.Add(step),
			})
		}
	}
	return dst
}
