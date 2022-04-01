package reporter

import (
	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/git"
	"github.com/cloudflare/pint/internal/parser"
)

type Report struct {
	Path          string
	ModifiedLines []int
	Rule          parser.Rule
	Problem       checks.Problem
	Owner         string
}

type Summary struct {
	Reports []Report
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

func blameReports(reports []Report, gitCmd git.CommandRunner) (pb git.FileBlames, err error) {
	pb = make(git.FileBlames)
	for _, report := range reports {
		if _, ok := pb[report.Path]; ok {
			continue
		}
		pb[report.Path], err = git.Blame(report.Path, gitCmd)
		if err != nil {
			return
		}
	}
	return
}
