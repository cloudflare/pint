package reporter_test

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/reporter"
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
	path := filepath.Join(t.TempDir(), "json-reporter-test.json")
	defer os.Remove(path)
	jsonReporter := reporter.NewJSONReporter(path)
	require.NoError(t, jsonReporter.Submit(reports))
	jsonFile, err := os.Open(path)
	require.NoError(t, err, "Couldn't open reported json file")
	defer jsonFile.Close()
	byteValue, err := io.ReadAll(jsonFile)
	require.NoError(t, err, "Error reading json")
	expected := "[{\"reportedPath\":\"\",\"sourcePath\":\"foo.txt\",\"rule\":{\"name\":\"sum errors\",\"type\":\"recording\"},\"problem\":{\"Fragment\":\"syntax error\",\"Lines\":[2],\"Reporter\":\"mock\",\"Text\":\"syntax error\",\"Severity\":\"Fatal\"},\"owner\":\"\"}]"
	require.Equal(t, expected, string(byteValue))
}
