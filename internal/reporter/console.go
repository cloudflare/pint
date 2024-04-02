package reporter

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"sort"
	"strings"

	"github.com/fatih/color"

	"github.com/cloudflare/pint/internal/checks"
)

func NewConsoleReporter(output io.Writer, minSeverity checks.Severity) ConsoleReporter {
	return ConsoleReporter{output: output, minSeverity: minSeverity}
}

type ConsoleReporter struct {
	output      io.Writer
	minSeverity checks.Severity
}

func (cr ConsoleReporter) Submit(summary Summary) (err error) {
	reports := summary.Reports()
	sort.Slice(reports, func(i, j int) bool {
		if reports[i].Path.Name < reports[j].Path.Name {
			return true
		}
		if reports[i].Path.Name > reports[j].Path.Name {
			return false
		}
		if reports[i].Problem.Lines.First < reports[j].Problem.Lines.First {
			return true
		}
		if reports[i].Problem.Lines.First > reports[j].Problem.Lines.First {
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

		if _, ok := perFile[report.Path.Name]; !ok {
			perFile[report.Path.Name] = []string{}
		}

		var content string
		if report.Problem.Anchor == checks.AnchorAfter {
			content, err = readFile(report.Path.Name)
			if err != nil {
				return err
			}
		}

		path := report.Path.Name
		if report.Path.Name != report.Path.SymlinkTarget {
			path = fmt.Sprintf("%s ~> %s", report.Path.Name, report.Path.SymlinkTarget)
		}
		path = color.CyanString("%s:%s", path, report.Problem.Lines)
		if report.Problem.Anchor == checks.AnchorBefore {
			path += " " + color.RedString("(deleted)")
		}
		path += " "

		msg := []string{path}
		switch report.Problem.Severity {
		case checks.Bug, checks.Fatal:
			msg = append(msg, color.RedString("%s: %s", report.Problem.Severity, report.Problem.Text))
		case checks.Warning:
			msg = append(msg, color.YellowString("%s: %s", report.Problem.Severity, report.Problem.Text))
		case checks.Information:
			msg = append(msg, color.HiBlackString("%s: %s", report.Problem.Severity, report.Problem.Text))
		}
		msg = append(msg, color.MagentaString(" (%s)\n", report.Problem.Reporter))

		if report.Problem.Anchor == checks.AnchorAfter {
			lines := strings.Split(content, "\n")
			lastLine := report.Problem.Lines.Last
			if lastLine > len(lines)-1 {
				lastLine = len(lines) - 1
				slog.Warn(
					"Tried to read more lines than present in the source file, this is likely due to '\n' usage in some rules, see https://github.com/cloudflare/pint/issues/20 for details",
					slog.String("path", report.Path.Name),
				)
			}

			nrFmt := fmt.Sprintf("%%%dd", countDigits(lastLine)+1)
			for i := report.Problem.Lines.First; i <= lastLine; i++ {
				msg = append(msg, color.WhiteString(nrFmt+" | %s\n", i, lines[i-1]))
			}
		}

		perFile[report.Path.Name] = append(perFile[report.Path.Name], strings.Join(msg, ""))
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

func countDigits(n int) (c int) {
	for n > 0 {
		n /= 10
		c++
	}
	return c
}
