package reporter

import (
	"time"

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
	OfflineChecks int64
	OnlineChecks  int64
	Duration      time.Duration
	Entries       int
	Reports       []Report
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

func reportedLine(report Report) (l int) {
	l = -1
	for _, pl := range report.Problem.Lines {
		for _, ml := range report.ModifiedLines {
			if pl == ml {
				l = pl
			}
		}
	}

	if l < 0 && report.Problem.Severity == checks.Fatal {
		for _, ml := range report.ModifiedLines {
			return ml
		}
	}

	return
}
