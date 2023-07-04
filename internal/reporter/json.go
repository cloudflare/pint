package reporter

import (
	"encoding/json"
	"os"
)

func NewJSONReporter(path string) JSONReporter {
	return JSONReporter{path}
}

type JSONReporter struct {
	path string
}

type JSONReport struct {
	ReportedPath: string	`json:reportedPath`
	SourcePath: string		`json:sourcePath`
	Rule: struct {
		Name: string		`json:name`
		Type: string		`json:type`

	}						`json:rule`
	Problem: checks.Problem	`json:problem`
	Owner: stringchan		`json:owner`
}

func (cr JSONReporter) Submit(reports []Report) error {
	result, err := json.Marshal(reports)
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
