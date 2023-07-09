package reporter

import (
	"encoding/json"
	"os"

	"github.com/cloudflare/pint/internal/checks"
)

func NewJSONReporter(path string) JSONReporter {
	return JSONReporter{path}
}

type JSONReporter struct {
	path string
}

type JSONReport struct {
	ReportedPath string         `json:"reportedPath"`
	SourcePath   string         `json:"sourcePath"`
	Rule         JSONReportRule `json:"rule"`
	Problem      checks.Problem `json:"problem"`
	Owner        string         `json:"owner"`
}

type JSONReportRule struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

func (cr JSONReporter) Submit(reports []Report) error {
	jsonReports := make([]JSONReport, 0, len(reports))
	for _, report := range reports {
		jsonReports = append(jsonReports, JSONReport{
			ReportedPath: report.ReportedPath,
			SourcePath:   report.SourcePath,
			Owner:        report.Owner,
			Problem:      report.Problem,
			Rule: JSONReportRule{
				Name: report.Rule.Name(),
				Type: string(report.Rule.Type()),
			},
		})
	}
	result, err := json.Marshal(jsonReports)
	if err != nil {
		return err
	}
	f, err := os.Create(cr.path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(string(result))
	return err
}
