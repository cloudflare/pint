package reporter

import (
	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"
)

type Report struct {
	Path    string
	Rule    parser.Rule
	Problem checks.Problem
}

func (r Report) IsPassing() bool {
	switch r.Problem.Severity {
	case checks.Information, checks.Warning:
		return true
	default:
		return false
	}
}

type Summary struct {
	Reports     []Report
	FileChanges discovery.FileFindResults
}

func (s Summary) IsPassing() bool {
	for _, r := range s.Reports {
		if p := r.IsPassing(); !p {
			return false
		}
	}
	return true
}

func (s Summary) HasFatalProblems() bool {
	for _, r := range s.Reports {
		if r.Problem.Severity == checks.Fatal {
			return true
		}
	}
	return false
}

func (s Summary) CountBySeverity() map[checks.Severity]int {
	m := map[checks.Severity]int{}
	for _, report := range s.Reports {
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
