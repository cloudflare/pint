package reporter

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/rs/zerolog/log"
	"golang.org/x/exp/slices"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/output"
)

func NewConsoleReporter(output io.Writer, minSeverity checks.Severity) ConsoleReporter {
	return ConsoleReporter{output: output, minSeverity: minSeverity}
}

type ConsoleReporter struct {
	output      io.Writer
	minSeverity checks.Severity
}

func (cr ConsoleReporter) Submit(summary Summary) error {
	reports := summary.Reports()
	sort.Slice(reports, func(i, j int) bool {
		if reports[i].SourcePath < reports[j].SourcePath {
			return true
		}
		if reports[i].SourcePath > reports[j].SourcePath {
			return false
		}
		if reports[i].Problem.Lines[0] < reports[j].Problem.Lines[0] {
			return true
		}
		if reports[i].Problem.Lines[0] > reports[j].Problem.Lines[0] {
			return false
		}
		if reports[i].Problem.Reporter < reports[j].Problem.Reporter {
			return true
		}
		if reports[i].Problem.Reporter > reports[j].Problem.Reporter {
			return false
		}
		return reports[i].Problem.Text < reports[j].Problem.Text
	})

	perFile := map[string][]string{}
	for _, report := range reports {
		if report.Problem.Severity < cr.minSeverity {
			continue
		}

		if !shouldReport(report) {
			log.Debug().
				Str("path", report.SourcePath).
				Str("lines", output.FormatLineRangeString(report.Problem.Lines)).
				Msg("Problem reported on unmodified line, skipping")
			continue
		}

		if _, ok := perFile[report.SourcePath]; !ok {
			perFile[report.SourcePath] = []string{}
		}

		content, err := readFile(report.SourcePath)
		if err != nil {
			return err
		}

		msg := []string{}

		firstLine, lastLine := report.Problem.LineRange()

		path := report.SourcePath
		if report.SourcePath != report.ReportedPath {
			path = fmt.Sprintf("%s ~> %s", report.SourcePath, report.ReportedPath)
		}

		msg = append(msg, color.CyanString("%s:%s ", path, printLineRange(firstLine, lastLine)))
		switch report.Problem.Severity {
		case checks.Bug, checks.Fatal:
			msg = append(msg, color.RedString("%s: %s", report.Problem.Severity, report.Problem.Text))
		case checks.Warning:
			msg = append(msg, color.YellowString("%s: %s", report.Problem.Severity, report.Problem.Text))
		case checks.Information:
			msg = append(msg, color.HiBlackString("%s: %s", report.Problem.Severity, report.Problem.Text))
		}
		msg = append(msg, color.MagentaString(" (%s)\n", report.Problem.Reporter))

		lines := strings.Split(content, "\n")
		if lastLine > len(lines)-1 {
			lastLine = len(lines) - 1
			log.Warn().Str("path", report.SourcePath).Msgf("Tried to read more lines than present in the source file, this is likely due to '\n' usage in some rules, see https://github.com/cloudflare/pint/issues/20 for details")
		}

		nrFmt := fmt.Sprintf("%%%dd", countDigits(lastLine)+1)
		var inPlaceholder bool
		for i := firstLine; i <= lastLine; i++ {
			switch {
			case slices.Contains(report.Problem.Lines, i):
				msg = append(msg, color.WhiteString(nrFmt+" | %s\n", i, lines[i-1]))
				inPlaceholder = false
			case inPlaceholder:
				//
			default:
				msg = append(msg, color.WhiteString(" %s\n", strings.Repeat(".", countDigits(lastLine))))
				inPlaceholder = true
			}
		}
		perFile[report.SourcePath] = append(perFile[report.SourcePath], strings.Join(msg, ""))
	}

	paths := []string{}
	for path := range perFile {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	for _, path := range paths {
		msgs := perFile[path]
		for _, msg := range msgs {
			fmt.Fprintln(cr.output, msg)
		}
	}

	return nil
}

func readFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	content, err := io.ReadAll(f)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

func printLineRange(s, e int) string {
	if s == e {
		return strconv.Itoa(s)
	}
	return fmt.Sprintf("%d-%d", s, e)
}

func countDigits(n int) (c int) {
	for n > 0 {
		n /= 10
		c++
	}
	return c
}
