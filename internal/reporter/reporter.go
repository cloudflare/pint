package reporter

import (
	"sort"
	"time"

	"golang.org/x/exp/slices"

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
	if !slices.Equal(r.Problem.Lines, nr.Problem.Lines) {
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
	OfflineChecks int64
	OnlineChecks  int64
	Duration      time.Duration
	Entries       int
	reports       []Report
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
		if s.reports[i].Problem.Lines[0] != s.reports[j].Problem.Lines[0] {
			return s.reports[i].Problem.Lines[0] < s.reports[j].Problem.Lines[0]
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
		if !shouldReport(report) {
			continue
		}
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

func shouldReport(report Report) bool {
	if report.Problem.Severity == checks.Fatal {
		return true
	}

	for _, pl := range report.Problem.Lines {
		for _, ml := range report.ModifiedLines {
			if pl == ml {
				return true
			}
		}
	}

	return false
}
