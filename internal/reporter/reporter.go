package reporter

import (
	"sort"
	"time"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/parser"
)

type Report struct {
	ReportedPath  string
	SourcePath    string
	ModifiedLines []int
	Rule          parser.Rule
	Problem       checks.Problem
	Owner         string
}

func (r Report) isEqual(nr Report) bool {
	if nr.ReportedPath != r.ReportedPath {
		return false
	}
	if nr.SourcePath != r.SourcePath {
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
	OfflineChecks  int64
	OnlineChecks   int64
	Duration       time.Duration
	TotalEntries   int
	CheckedEntries int64
	reports        []Report
}

func NewSummary(reports []Report) Summary {
	return Summary{reports: reports}
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
	sort.SliceStable(s.reports, func(i, j int) bool {
		if s.reports[i].ReportedPath != s.reports[j].ReportedPath {
			return s.reports[i].ReportedPath < s.reports[j].ReportedPath
		}
		if s.reports[i].SourcePath != s.reports[j].SourcePath {
			return s.reports[i].SourcePath < s.reports[j].SourcePath
		}
		if s.reports[i].Problem.Lines.First != s.reports[j].Problem.Lines.First {
			return s.reports[i].Problem.Lines.First < s.reports[j].Problem.Lines.First
		}
		if s.reports[i].Problem.Reporter != s.reports[j].Problem.Reporter {
			return s.reports[i].Problem.Reporter < s.reports[j].Problem.Reporter
		}
		return s.reports[i].Problem.Text < s.reports[j].Problem.Text
	})
}

func (s Summary) Reports() (reports []Report) {
	return s.reports
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
