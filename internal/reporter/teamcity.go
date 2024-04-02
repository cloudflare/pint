package reporter

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/cloudflare/pint/internal/checks"
)

func NewTeamCityReporter(output io.Writer) TeamCityReporter {
	return TeamCityReporter{
		output: output,
		escaper: strings.NewReplacer(
			"'", "|'",
			"\n", "|n",
			"\r", "|r",
			"\\uNNNN", "|0xNNNN",
			"|", "||",
			"[", "|[",
			"]", "|]",
		),
	}
}

type TeamCityReporter struct {
	output  io.Writer
	escaper *strings.Replacer
}

func (tc TeamCityReporter) name(report Report) string {
	return fmt.Sprintf("%s:%d", report.Path.SymlinkTarget, report.Problem.Lines.First)
}

func (tc TeamCityReporter) escape(s string) string {
	return tc.escaper.Replace(s)
}

func (tc TeamCityReporter) Submit(summary Summary) error {
	var buf strings.Builder
	for _, report := range summary.reports {
		buf.WriteString("##teamcity[testSuiteStarted name='")
		buf.WriteString(report.Problem.Reporter)
		buf.WriteString("']\n")

		buf.WriteString("##teamcity[testSuiteStarted name='")
		buf.WriteString(report.Problem.Severity.String())
		buf.WriteString("']\n")

		buf.WriteString("##teamcity[testStarted name='")
		buf.WriteString(tc.name(report))
		buf.WriteString("']\n")

		if report.Problem.Severity >= checks.Bug {
			buf.WriteString("##teamcity[testFailed name='")
			buf.WriteString(tc.name(report))
			buf.WriteString("' message='' details='")
			buf.WriteString(tc.escape(report.Problem.Text))
			buf.WriteString("']\n")
		} else {
			buf.WriteString("##teamcity[testStdErr name='")
			buf.WriteString(tc.name(report))
			buf.WriteString("' out='")
			buf.WriteString(tc.escape(report.Problem.Text))
			buf.WriteString("']\n")
		}

		buf.WriteString("##teamcity[testFinished name='")
		buf.WriteString(report.Path.SymlinkTarget)
		buf.WriteRune(':')
		buf.WriteString(strconv.Itoa(report.Problem.Lines.First))
		buf.WriteString("']\n")

		buf.WriteString("##teamcity[testSuiteFinished name='")
		buf.WriteString(report.Problem.Severity.String())
		buf.WriteString("']\n")

		buf.WriteString("##teamcity[testSuiteFinished name='")
		buf.WriteString(report.Problem.Reporter)
		buf.WriteString("']\n")

		fmt.Fprint(tc.output, buf.String())
		buf.Reset()
	}
	return nil
}
