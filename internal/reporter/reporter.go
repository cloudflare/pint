package reporter

import (
	"cmp"
	"slices"
	"time"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"
)

type Report struct {
	Path          discovery.Path
	Owner         string
	ModifiedLines []int
	Rule          parser.Rule
	Problem       checks.Problem
}

func (r Report) isEqual(nr Report) bool {
	if nr.Path.SymlinkTarget != r.Path.SymlinkTarget {
		return false
	}
	if nr.Path.Name != r.Path.Name {
		return false
	}
	if nr.Owner != r.Owner {
		return false
	}
	if r.Problem.Lines.First != nr.Problem.Lines.First {
		return false
	}
	if r.Problem.Lines.Last != nr.Rule.Lines.Last {
		return false
	}
	if !nr.Rule.IsSame(r.Rule) {
		return false
	}
	if nr.Problem.Reporter != r.Problem.Reporter {
		return false
	}
	if nr.Problem.Text != r.Problem.Text {
		return false
	}
	if nr.Problem.Severity != r.Problem.Severity {
		return false
	}
	return true
}

type Summary struct {
	reports        []Report
	OfflineChecks  int64
	OnlineChecks   int64
	Duration       time.Duration
	TotalEntries   int
	CheckedEntries int64
}

func NewSummary(reports []Report) Summary {
	return Summary{reports: reports} // nolint: exhaustruct
}

func (s *Summary) Report(reps ...Report) {
	for _, r := range reps {
		if !s.hasReport(r) {
			s.reports = append(s.reports, r)
		}
	}
}

func (s Summary) hasReport(r Report) bool {
	for _, er := range s.reports {
		if er.isEqual(r) {
			return true
		}
	}
	return false
}

func (s *Summary) SortReports() {
	slices.SortFunc(s.reports, func(a, b Report) int {
		return cmp.Or(
			cmp.Compare(a.Path.Name, b.Path.Name),
			cmp.Compare(a.Problem.Lines.First, b.Problem.Lines.First),
			cmp.Compare(a.Problem.Lines.Last, b.Problem.Lines.Last),
			cmp.Compare(a.Problem.Severity, b.Problem.Severity),
			cmp.Compare(a.Problem.Reporter, b.Problem.Reporter),
			cmp.Compare(a.Problem.Text, b.Problem.Text),
		)
	})
}

func (s Summary) Reports() (reports []Report) {
	return s.reports
}

func (s Summary) ReportsPerPath() (reports [][]Report) {
	curPath := []Report{}
	for _, r := range s.reports {
		if len(curPath) > 0 && curPath[0].Path.Name != r.Path.Name {
			reports = append(reports, curPath)
			curPath = []Report{}
		}
		curPath = append(curPath, r)
	}
	reports = append(reports, curPath)
	return reports
}

func (s Summary) HasFatalProblems() bool {
	for _, r := range s.Reports() {
		if r.Problem.Severity == checks.Fatal {
			return true
		}
	}
	return false
}

func (s Summary) CountBySeverity() map[checks.Severity]int {
	m := map[checks.Severity]int{}
	for _, report := range s.Reports() {
		if _, ok := m[report.Problem.Severity]; !ok {
			m[report.Problem.Severity] = 0
		}
		m[report.Problem.Severity]++
	}
	return m
}

type Reporter interface {
	Submit(Summary) error
}
