package main

import (
	"fmt"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func mockRules(dir string, filesCount, rulesPerFile int) error {
	var rulePath, c string
	var err error
	var content strings.Builder
	for i := 1; i <= filesCount; i++ {
		content.Reset()
		rulePath = path.Join(dir, fmt.Sprintf("%d_rules.yaml", i))
		for j := 1; j <= rulesPerFile; j++ {
			c = fmt.Sprintf("- record: %d_rule\n  expr: sum(foo) without(instance)\n  labels:\n    foo: bar\n\n", j)
			if _, err = content.WriteString(c); err != nil {
				return err
			}
		}

		if err = os.WriteFile(rulePath, []byte(content.String()), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func mockConfig(configPath string) error {
	content := `
parser {
  relaxed = ["(.*)"]
}
rule {
  reject ".* +.*" {
    label_keys      = true
    annotation_keys = true
  }

  reject "https?://.+" {
    label_keys   = true
    label_values = true
  }
}

rule {
  match {
    kind = "alerting"
  }

  annotation "summary" {
    severity = "bug"
    required = true
  }

  annotation "dashboard" {
    severity = "bug"
    value    = "https://grafana.example.com/(.+)"
  }

  label "priority" {
    severity = "bug"
    value    = "(info|warning|critical)"
    required = true
  }

  label "notify" {
    severity = "bug"
    required = true
  }

  label "component" {
    severity = "bug"
    required = true
  }

  alerts {
    range   = "1d"
    step    = "1m"
    resolve = "5m"
  }
}

rule {
  match {
    kind = "alerting"

    label "notify" {
      value = "(?:.*\\s+)?(chat|pagerduty|jira)(?:\\s+.*)?"
    }
  }

  annotation "link" {
    severity = "bug"
    value    = "https://alert-references.(?:s3.)?cfdata.org/(.+)"
    required = true
  }
}

rule {
  match {
    kind = "recording"
  }

  aggregate ".+" {
    severity = "bug"
    keep     = ["job"]
  }

  cost {}
}

rule {
  match {
    kind = "recording"
    path = ".*"
  }

  aggregate "dc(?:_.+)?:.+" {
    severity = "bug"
    strip    = ["instance"]
  }
}    
`
	return os.WriteFile(configPath, []byte(content), 0o644)
}

func BenchmarkLint(b *testing.B) {
	var err error

	rulesDir := b.TempDir()
	if err = mockRules(rulesDir, 100, 50); err != nil {
		b.Error(err)
		b.FailNow()
	}

	configPath := path.Join(rulesDir, ".pint.hcl")
	if err = mockConfig(configPath); err != nil {
		b.Error(err)
		b.FailNow()
	}

	app := newApp()
	args := []string{"pint", "-c", configPath, "-l", "error", "--offline", "lint", rulesDir + "/*.yaml"}
	for n := 0; n < b.N; n++ {
		if err = app.Run(args); err != nil {
			b.Error(err)
			b.FailNow()
		}
	}
}

func TestLintReporters(t *testing.T) {
	var err error

	rulesDir := t.TempDir()
	err = mockRules(rulesDir, 1, 1)
	require.NoError(t, err)
	configPath := path.Join(rulesDir, ".pint.hcl")
	err = mockConfig(configPath)
	require.NoError(t, err)

	json_file := path.Join(rulesDir, ".reporter.json")
	content := fmt.Sprintf(`
  parser {
    relaxed = ["(.*)"]
  }
  
  reporters {
    json {
      path = "%s"
    }
  }
  `, strings.Replace(json_file, `\`, `\\`, -1))
	os.WriteFile(configPath, []byte(content), 0o644)
	app := newApp()
	err = app.Run([]string{"pint", "-c", configPath, "-l", "error", "--offline", "lint", rulesDir + "/*.yaml"})
	require.NoError(t, err)
	require.FileExists(t, json_file)
}
