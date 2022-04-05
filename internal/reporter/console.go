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

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/output"
)

func NewConsoleReporter(output io.Writer) ConsoleReporter {
	return ConsoleReporter{output: output}
}

type ConsoleReporter struct {
	output io.Writer
}

func (cr ConsoleReporter) Submit(summary Summary) error {
	reports := summary.Reports
	reps := reports[:]
	sort.Slice(reps, func(i, j int) bool {
		if reports[i].Path < reports[j].Path {
			return true
		}
		if reports[i].Path > reports[j].Path {
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
	for _, report := range reps {
		if !shouldReport(report) {
			log.Debug().
				Str("path", report.Path).
				Str("lines", output.FormatLineRangeString(report.Problem.Lines)).
				Msg("Problem reported on unmodified line, skipping")
			continue
		}

		if _, ok := perFile[report.Path]; !ok {
			perFile[report.Path] = []string{}
		}

		content, err := readFile(report.Path)
		if err != nil {
			return err
		}

		msg := []string{}
		firstLine, lastLine := report.Problem.LineRange()
		msg = append(msg, color.CyanString("%s:%s: ", report.Path, printLineRange(firstLine, lastLine)))
		switch report.Problem.Severity {
		case checks.Bug, checks.Fatal:
			msg = append(msg, color.RedString(report.Problem.Text))
		case checks.Warning:
			msg = append(msg, color.YellowString(report.Problem.Text))
		default:
			msg = append(msg, color.HiBlackString(report.Problem.Text))
		}
		msg = append(msg, color.MagentaString(" (%s)\n", report.Problem.Reporter))

		lines := strings.Split(content, "\n")
		if lastLine > len(lines)-1 {
			lastLine = len(lines) - 1
			log.Warn().Str("path", report.Path).Msgf("Tried to read more lines than present in the source file, this is likely due to '\n' usage in some rules, see https://github.com/cloudflare/pint/issues/20 for details")
		}
		for _, c := range lines[firstLine-1 : lastLine] {
			msg = append(msg, color.WhiteString("%s\n", c))
		}
		perFile[report.Path] = append(perFile[report.Path], strings.Join(msg, ""))
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
