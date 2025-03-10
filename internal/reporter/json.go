package reporter

import (
	"encoding/json"
	"io"
)

func NewJSONReporter(output io.Writer) JSONReporter {
	return JSONReporter{output: output}
}

type JSONReporter struct {
	output io.Writer
}

type JSONReport struct {
	Path     string `json:"path"`
	Owner    string `json:"owner,omitempty"`
	Reporter string `json:"reporter"`
	Problem  string `json:"problem"`
	Details  string `json:"details,omitempty"`
	Severity string `json:"severity"`
	Lines    []int  `json:"lines"`
}

func (jr JSONReporter) Submit(summary Summary) (err error) {
	reports := summary.Reports()
	out := make([]JSONReport, 0, len(reports))

	for _, report := range reports {
		out = append(out, JSONReport{
			Path:     report.Path.Name,
			Owner:    report.Owner,
			Reporter: report.Problem.Reporter,
			Problem:  report.Problem.Summary,
			Details:  report.Problem.Details,
			Severity: report.Problem.Severity.String(),
			Lines:    report.Problem.Lines.Expand(),
		})
	}

	enc := json.NewEncoder(jr.output)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
