package reporter

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/fatih/color"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/output"
)

func NewConsoleReporter(output io.Writer, minSeverity checks.Severity, noColor bool) ConsoleReporter {
	return ConsoleReporter{
		output:      output,
		minSeverity: minSeverity,
		noColor:     noColor,
	}
}

type ConsoleReporter struct {
	output      io.Writer
	minSeverity checks.Severity
	noColor     bool
}

func (cr ConsoleReporter) Submit(summary Summary) (err error) {
	var buf strings.Builder
	var content string
	for _, reports := range summary.ReportsPerPath() {
		content = ""
		for _, report := range reports {
			if report.Problem.Severity < cr.minSeverity {
				continue
			}
			if content == "" && report.Problem.Anchor == checks.AnchorAfter {
				content, err = readFile(report.Path.Name)
				if err != nil {
					return err
				}
			}
			buf.Reset()

			buf.WriteString(output.MaybeColor(color.CyanString, cr.noColor, report.Path.Name)) // nolint:govet
			if report.Path.Name != report.Path.SymlinkTarget {
				buf.WriteString(output.MaybeColor(color.CyanString, cr.noColor, " ~> %s", report.Path.SymlinkTarget))
			}
			buf.WriteString(output.MaybeColor(color.CyanString, cr.noColor, ":%s", report.Problem.Lines.String()))
			if report.Problem.Anchor == checks.AnchorBefore {
				buf.WriteRune(' ')
				buf.WriteString(output.MaybeColor(color.RedString, cr.noColor, "(deleted)"))
			}
			buf.WriteRune(' ')

			switch report.Problem.Severity {
			case checks.Bug, checks.Fatal:
				buf.WriteString(output.MaybeColor(color.RedString, cr.noColor, "%s: %s", report.Problem.Severity, report.Problem.Text))
			case checks.Warning:
				buf.WriteString(output.MaybeColor(color.YellowString, cr.noColor, "%s: %s", report.Problem.Severity, report.Problem.Text))
			case checks.Information:
				buf.WriteString(output.MaybeColor(color.HiBlackString, cr.noColor, "%s: %s", report.Problem.Severity, report.Problem.Text))
			}
			buf.WriteString(output.MaybeColor(color.MagentaString, cr.noColor, " (%s)\n", report.Problem.Reporter))

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
					buf.WriteString(output.MaybeColor(color.WhiteString, cr.noColor, nrFmt+" | %s\n", i, lines[i-1]))
				}
			}

			fmt.Fprintln(cr.output, buf.String())
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
