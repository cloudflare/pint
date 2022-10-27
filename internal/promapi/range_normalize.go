package promapi

import (
	"sort"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
)

func labelValue(ls labels.Labels, name string) (string, bool) {
	for _, l := range ls {
		if l.Name == name {
			return l.Value, true
		}
	}
	return "", false
}

func labelsBefore(ls, o labels.Labels) bool {
	if len(ls) < len(o) {
		return true
	}
	if len(ls) > len(o) {
		return false
	}

	lns := make([]string, 0, len(ls)+len(o))
	for _, ln := range ls {
		lns = append(lns, ln.Name)
	}
	for _, ln := range o {
		lns = append(lns, ln.Name)
	}
	sort.Strings(lns)
	for _, ln := range lns {
		mlv, ok := labelValue(ls, ln)
		if !ok {
			return true
		}
		olv, ok := labelValue(o, ln)
		if !ok {
			return false
		}
		if mlv < olv {
			return true
		}
		if mlv > olv {
			return false
		}
	}
	return false
}

type TimeRange struct {
	Start time.Time
	End   time.Time
}

type MetricTimeRange struct {
	Fingerprint uint64
	Labels      labels.Labels
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
		return labelsBefore(mtr[i].Labels, mtr[j].Labels)
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

	toPurge := map[int]struct{}{}
	var ok bool
	for i := range dst {
		for j := range dst {
			if i == j {
				continue
			}
			if dst[i].Fingerprint != dst[j].Fingerprint {
				continue
			}
			if _, ok = toPurge[j]; ok {
				continue
			}
			if dst[i].Start.Before(dst[j].Start) && !dst[i].End.Before(dst[j].Start) && !dst[i].End.After(dst[j].End) {
				dst[i].End = dst[j].End
				toPurge[j] = struct{}{}
			}
		}
	}

	merged := make(MetricTimeRanges, 0, len(dst)-len(toPurge))
	for i, tr := range dst {
		if _, ok = toPurge[i]; ok {
			goto NEXT
		}
		merged = append(merged, tr)
	NEXT:
	}

	return merged
}

func AppendSampleToRanges(dst MetricTimeRanges, ls labels.Labels, vals []model.SamplePair, step time.Duration) MetricTimeRanges {
	fp := ls.Hash()

	var ts time.Time
	var found bool
	for _, v := range vals {
		ts = v.Timestamp.Time()
		found = false
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

func MetricToLabels(m model.Metric) labels.Labels {
	lset := make([]string, 0, len(m)*2)
	for k, v := range m {
		lset = append(lset, string(k), string(v))
	}
	return labels.FromStrings(lset...)
}
