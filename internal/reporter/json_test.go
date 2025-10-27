package reporter_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/reporter"
)

func TestJSONReporter(t *testing.T) {
	t.Run("NewJSONReporter creates reporter", func(t *testing.T) {
		buf := &bytes.Buffer{}
		jr := reporter.NewJSONReporter(buf)
		require.NotNil(t, jr)
	})

	t.Run("Submit with empty summary", func(t *testing.T) {
		buf := &bytes.Buffer{}
		jr := reporter.NewJSONReporter(buf)

		summary := reporter.NewSummary([]reporter.Report{})
		err := jr.Submit(context.Background(), summary)
		require.NoError(t, err)

		var result []reporter.JSONReport
		err = json.Unmarshal(buf.Bytes(), &result)
		require.NoError(t, err)
		require.Empty(t, result)
	})

	t.Run("Submit with single report", func(t *testing.T) {
		buf := &bytes.Buffer{}
		jr := reporter.NewJSONReporter(buf)

		p := parser.NewParser(false, parser.PrometheusSchema, model.UTF8Validation)
		mockFile := p.Parse(strings.NewReader(`
- record: test
  expr: up == 0
`))

		summary := reporter.NewSummary([]reporter.Report{
			{
				Path: discovery.Path{
					Name:          "test.yml",
					SymlinkTarget: "test.yml",
				},
				Owner:         "team-a",
				ModifiedLines: []int{2},
				Rule:          mockFile.Groups[0].Rules[0],
				Problem: checks.Problem{
					Lines: diags.LineRange{
						First: 1,
						Last:  2,
					},
					Reporter: "test-reporter",
					Summary:  "test error",
					Details:  "test details",
					Severity: checks.Fatal,
				},
			},
		})

		err := jr.Submit(context.Background(), summary)
		require.NoError(t, err)

		var result []reporter.JSONReport
		err = json.Unmarshal(buf.Bytes(), &result)
		require.NoError(t, err)
		require.Len(t, result, 1)
		require.Equal(t, "test.yml", result[0].Path)
		require.Equal(t, "team-a", result[0].Owner)
		require.Equal(t, "test-reporter", result[0].Reporter)
		require.Equal(t, "test error", result[0].Problem)
		require.Equal(t, "test details", result[0].Details)
		require.Equal(t, "Fatal", result[0].Severity)
		require.Equal(t, []int{1, 2}, result[0].Lines)
	})

	t.Run("Submit with multiple reports", func(t *testing.T) {
		buf := &bytes.Buffer{}
		jr := reporter.NewJSONReporter(buf)

		p := parser.NewParser(false, parser.PrometheusSchema, model.UTF8Validation)
		mockFile := p.Parse(strings.NewReader(`
- record: test1
  expr: up == 0
- record: test2
  expr: down == 1
`))

		summary := reporter.NewSummary([]reporter.Report{
			{
				Path: discovery.Path{
					Name:          "test1.yml",
					SymlinkTarget: "test1.yml",
				},
				Rule: mockFile.Groups[0].Rules[0],
				Problem: checks.Problem{
					Lines:    diags.LineRange{First: 1, Last: 1},
					Reporter: "reporter1",
					Summary:  "error1",
					Severity: checks.Warning,
				},
			},
			{
				Path: discovery.Path{
					Name:          "test2.yml",
					SymlinkTarget: "test2.yml",
				},
				Rule: mockFile.Groups[0].Rules[1],
				Problem: checks.Problem{
					Lines:    diags.LineRange{First: 3, Last: 5},
					Reporter: "reporter2",
					Summary:  "error2",
					Details:  "details2",
					Severity: checks.Bug,
				},
			},
		})

		err := jr.Submit(context.Background(), summary)
		require.NoError(t, err)

		var result []reporter.JSONReport
		err = json.Unmarshal(buf.Bytes(), &result)
		require.NoError(t, err)
		require.Len(t, result, 2)
		require.Equal(t, "test1.yml", result[0].Path)
		require.Equal(t, "test2.yml", result[1].Path)
	})

	t.Run("Submit with report without owner and details", func(t *testing.T) {
		buf := &bytes.Buffer{}
		jr := reporter.NewJSONReporter(buf)

		p := parser.NewParser(false, parser.PrometheusSchema, model.UTF8Validation)
		mockFile := p.Parse(strings.NewReader(`
- record: test
  expr: up
`))

		summary := reporter.NewSummary([]reporter.Report{
			{
				Path: discovery.Path{
					Name:          "test.yml",
					SymlinkTarget: "test.yml",
				},
				Rule: mockFile.Groups[0].Rules[0],
				Problem: checks.Problem{
					Lines:    diags.LineRange{First: 1, Last: 1},
					Reporter: "test",
					Summary:  "test",
					Severity: checks.Information,
				},
			},
		})

		err := jr.Submit(context.Background(), summary)
		require.NoError(t, err)

		var result []reporter.JSONReport
		err = json.Unmarshal(buf.Bytes(), &result)
		require.NoError(t, err)
		require.Len(t, result, 1)
		require.Empty(t, result[0].Owner)
		require.Empty(t, result[0].Details)
	})
}
