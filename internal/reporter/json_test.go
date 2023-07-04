package reporter_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/reporter"
	"github.com/stretchr/testify/require"
)

func TestJSONReporter(t *testing.T) {
	p := parser.NewParser()
	mockRules, _ := p.Parse([]byte(`
- record: target is down
  expr: up == 0
- record: sum errors
  expr: sum(errors) by (job)
`))
	reports := []reporter.Report{
		{
			SourcePath:    "foo.txt",
			ModifiedLines: []int{2},
			Rule:          mockRules[1],
			Problem: checks.Problem{
				Fragment: "syntax error",
				Lines:    []int{2},
				Reporter: "mock",
				Text:     "syntax error",
				Severity: checks.Fatal,
			},
		},
	}
	path := filepath.Join(os.TempDir(), "json-reporter-test.json")
	//defer os.Remove(path)
	fmt.Println(path)
	json_reporter := reporter.NewJSONReporter(path)
	json_reporter.Submit(reports)
	jsonFile, err := os.Open(path)
	require.NoError(t, err, "Couldn't open reported json file")
	defer jsonFile.Close()
	byteValue, _ := ioutil.ReadAll(jsonFile)
	var readReports []reporter.Report
	err = json.Unmarshal(byteValue, &readReports)
	require.NoError(t, err, "Error marshalling json")
	require.Equal(t, reports, readReports)

}
