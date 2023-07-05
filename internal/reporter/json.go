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
	json_reports := make([]JSONReport, 0, len(reports))
	for _, report := range reports {
		json_reports = append(json_reports, JSONReport{
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
	result, err := json.Marshal(json_reports)
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
