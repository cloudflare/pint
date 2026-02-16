package reporter

import (
	"cmp"
	"context"
	"slices"
	"time"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"
)

type Report struct {
	Path          discovery.Path
	Owner         string
	ModifiedLines []int
	Duplicates    []*Report
	Rule          parser.Rule
	Problem       checks.Problem
	IsDuplicate   bool
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
	if nr.Problem.Summary != r.Problem.Summary {
		return false
	}
	if !isSameDiagnostics(nr.Problem.Diagnostics, r.Problem.Diagnostics) {
		return false
	}
	if nr.Problem.Severity != r.Problem.Severity {
		return false
	}
	return true
}

func (r Report) isSameIssue(nr Report) bool {
	if nr.Problem.Reporter != r.Problem.Reporter {
		return false
	}
	if nr.Problem.Summary != r.Problem.Summary {
		return false
	}
	if nr.Problem.Severity != r.Problem.Severity {
		return false
	}
	if !isSameDiagnosticsMessage(r.Problem.Diagnostics, nr.Problem.Diagnostics) {
		return false
	}
	return true
}

type DisabledChecks struct {
	API    string
	Checks []string
}

type PrometheusDetails struct {
	Name           string
	DisabledChecks []DisabledChecks
}

type Summary struct {
	promDetails    map[string]PrometheusDetails
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

func (s *Summary) MarkCheckDisabled(prom, api string, checks []string) {
	if s.promDetails == nil {
		s.promDetails = map[string]PrometheusDetails{}
	}
	if _, ok := s.promDetails[prom]; !ok {
		s.promDetails[prom] = PrometheusDetails{} // nolint: exhaustruct
	}
	s.promDetails[prom] = PrometheusDetails{
		Name:           prom,
		DisabledChecks: append(s.promDetails[prom].DisabledChecks, DisabledChecks{API: api, Checks: checks}),
	}
}

func (s *Summary) GetPrometheusDetails() []PrometheusDetails {
	pd := make([]PrometheusDetails, 0, len(s.promDetails))
	for _, p := range s.promDetails {
		for idx := range p.DisabledChecks {
			slices.Sort(p.DisabledChecks[idx].Checks)
		}
		slices.SortStableFunc(p.DisabledChecks, func(a, b DisabledChecks) int {
			return cmp.Compare(a.API, b.API)
		})
		pd = append(pd, p)
	}
	slices.SortStableFunc(pd, func(a, b PrometheusDetails) int {
		return cmp.Compare(a.Name, b.Name)
	})
	return pd
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
	for i := range s.reports {
		slices.SortStableFunc(s.reports[i].Problem.Diagnostics, func(a, b diags.Diagnostic) int {
			return cmp.Or(
				cmp.Compare(b.FirstColumn, a.FirstColumn),
				cmp.Compare(a.LastColumn, b.LastColumn),
				cmp.Compare(a.Message, b.Message),
			)
		})
	}

	slices.SortStableFunc(s.reports, func(a, b Report) int {
		return cmp.Or(
			cmp.Compare(a.Path.Name, b.Path.Name),
			cmp.Compare(a.Problem.Lines.First, b.Problem.Lines.First),
			cmp.Compare(a.Problem.Lines.Last, b.Problem.Lines.Last),
			cmp.Compare(a.Problem.Severity, b.Problem.Severity),
			cmp.Compare(a.Problem.Reporter, b.Problem.Reporter),
			cmp.Compare(a.Problem.Summary, b.Problem.Summary),
			cmpDiagnostics(a.Problem.Diagnostics, b.Problem.Diagnostics),
		)
	})
}

func (s *Summary) Dedup() {
	for i := range s.reports {
		if s.reports[i].IsDuplicate {
			continue
		}
		for j := range s.reports {
			if i == j {
				continue
			}
			if s.reports[j].IsDuplicate {
				continue
			}
			if len(s.reports[j].Duplicates) > 0 {
				continue
			}
			if s.reports[i].isSameIssue(s.reports[j]) {
				s.reports[j].IsDuplicate = true
				s.reports[i].Duplicates = append(s.reports[i].Duplicates, &s.reports[j])
			}
		}
	}
}

func (s Summary) Reports() (reports []Report) {
	return s.reports
}

func (s Summary) ReportsPerPath() (reports [][]Report) {
	curPath := make([]Report, 0, len(s.reports))
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
	Submit(context.Context, Summary) error
}

func cmpDiags(a, b diags.Diagnostic) int {
	return cmp.Or(
		cmp.Compare(b.FirstColumn, a.FirstColumn),
		cmp.Compare(a.LastColumn, b.LastColumn),
		cmp.Compare(a.Message, b.Message),
	)
}

func cmpDiagnostics(sa, sb []diags.Diagnostic) int {
	if len(sa) == 0 {
		return -1
	}
	if len(sb) == 0 {
		return 1
	}

	slices.SortStableFunc(sa, cmpDiags)
	slices.SortStableFunc(sb, cmpDiags)

	return cmpDiags(sa[0], sb[0])
}

func isSameDiagnostics(sa, sb []diags.Diagnostic) bool {
	if len(sa) != len(sb) {
		return false
	}

	var ok bool
	for _, a := range sa {
		ok = false
		for _, b := range sb {
			if a.FirstColumn == b.FirstColumn && a.LastColumn == b.LastColumn && a.Message == b.Message {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}

	return true
}

func isSameDiagnosticsMessage(sa, sb []diags.Diagnostic) bool {
	if len(sa) != len(sb) {
		return false
	}

	var ok bool
	for _, a := range sa {
		ok = false
		for _, b := range sb {
			if a.Message == b.Message {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}

	return true
}
