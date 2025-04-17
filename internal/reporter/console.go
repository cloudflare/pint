package reporter

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/output"
)

func NewConsoleReporter(output io.Writer, minSeverity checks.Severity, noColor, showDuplicates bool) ConsoleReporter {
	return ConsoleReporter{
		output:         output,
		minSeverity:    minSeverity,
		noColor:        noColor,
		showDuplicates: showDuplicates,
	}
}

type ConsoleReporter struct {
	output         io.Writer
	minSeverity    checks.Severity
	noColor        bool
	showDuplicates bool
}

func (cr ConsoleReporter) Submit(summary Summary) (err error) {
	var buf strings.Builder
	var content string
	var allReports, hiddenDupes int
	for _, reports := range summary.ReportsPerPath() {
		content = ""
		for _, report := range reports {
			if report.Problem.Severity < cr.minSeverity {
				continue
			}
			allReports++
			if !cr.showDuplicates && report.IsDuplicate {
				hiddenDupes++
				continue
			}
			if content == "" && report.Problem.Anchor == checks.AnchorAfter {
				content, err = readFile(report.Path.Name)
				if err != nil {
					return err
				}
			}
			buf.Reset()

			var color output.Color
			switch {
			case cr.noColor:
				color = output.None
			case report.Problem.Severity == checks.Bug:
				color = output.Red
			case report.Problem.Severity == checks.Fatal:
				color = output.Red
			case report.Problem.Severity == checks.Warning:
				color = output.Yellow
			case report.Problem.Severity == checks.Information:
				color = output.Dim
			}

			buf.WriteString(output.MaybeColor(color, cr.noColor, report.Problem.Severity.String()+": "))
			buf.WriteString(output.MaybeColor(output.Bold, cr.noColor, report.Problem.Summary))
			buf.WriteString(output.MaybeColor(output.Magenta, cr.noColor, " ("+report.Problem.Reporter+")\n"))

			buf.WriteString(output.MaybeColor(output.Cyan, cr.noColor, "  ---> "+report.Path.Name))
			if report.Path.Name != report.Path.SymlinkTarget {
				buf.WriteString(output.MaybeColor(output.Cyan, cr.noColor, " ~> "+report.Path.SymlinkTarget))
			}
			buf.WriteString(output.MaybeColor(output.Cyan, cr.noColor, ":"+report.Problem.Lines.String()))
			if report.Problem.Anchor == checks.AnchorBefore {
				buf.WriteString(output.MaybeColor(output.Red, cr.noColor, " (deleted)"))
			}
			if report.Rule.Name() != "" {
				buf.WriteString(output.MaybeColor(output.Bold, cr.noColor, " -> `"+report.Rule.Name()+"`"))
			}
			if !cr.showDuplicates && len(report.Duplicates) > 0 {
				buf.WriteRune(' ')
				buf.WriteString(output.MaybeColor(output.Blue, cr.noColor, "[+"+strconv.Itoa(len(report.Duplicates))+" duplicates]"))
			}
			buf.WriteRune('\n')

			if report.Problem.Anchor == checks.AnchorAfter {
				if len(report.Problem.Diagnostics) > 0 {
					body := diags.InjectDiagnostics(
						content,
						report.Problem.Diagnostics,
						color,
					)
					buf.WriteString(output.MaybeColor(output.White, cr.noColor, body))
				} else {
					digits := countDigits(report.Problem.Lines.Last) + 1
					lines := strings.Split(content, "\n")
					nrFmt := fmt.Sprintf("%%%dd", digits)
					for i := report.Problem.Lines.First; i <= report.Problem.Lines.Last; i++ {
						buf.WriteString(output.MaybeColor(output.White, cr.noColor, fmt.Sprintf(nrFmt+" | %s\n", i, lines[i-1])))
					}
					buf.WriteString(strings.Repeat(" ", digits+3))
					buf.WriteString(output.MaybeColor(color, cr.noColor, "^^^ "+report.Problem.Summary))
					buf.WriteRune('\n')
				}
			}

			fmt.Fprintln(cr.output, buf.String())
		}
	}

	if hiddenDupes > 0 {
		slog.Info(
			"Some problems are duplicated between rules and all the duplicates were hidden, pass `--show-duplicates` to see them",
			slog.Int("total", allReports),
			slog.Int("duplicates", hiddenDupes),
			slog.Int("shown", allReports-hiddenDupes),
		)
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
