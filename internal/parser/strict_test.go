package parser

import (
	"io"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestValidateRuleFile(t *testing.T) {
	type testCaseT struct {
		content string
		err     string
		line    int
	}

	testCases := []testCaseT{
		{
			content: "[]",
			err:     "YAML list is not allowed here, expected a YAML mapping",
			line:    1,
		},
		{
			content: "\n\n[]",
			err:     "YAML list is not allowed here, expected a YAML mapping",
			line:    3,
		},
		{
			content: "groups: {}",
			err:     "YAML mapping is not allowed here, expected a YAML list",
			line:    1,
		},
		{
			content: "groups: []",
		},
		{
			content: "xgroups: {}",
			err:     "unexpected key `xgroups`",
			line:    1,
		},
		{
			content: "\nbob\n",
			err:     "YAML scalar value is not allowed here, expected a YAML mapping",
			line:    2,
		},
		{
			content: `groups: []

rules: []
`,
			err:  "unexpected key `rules`",
			line: 3,
		},
		{
			content: `
groups:
- name: foo
  rules: []
`,
		},
		{
			content: `
groups:
- name: 
  rules: []
`,
			err:  "expected a YAML string here, got null instead",
			line: 3,
		},
		{
			content: `
groups:
- name: foo
  rules:
  - record: foo
    expr: sum(up)
    labels:
      job: foo
`,
		},
		{
			content: `
groups:
- name: foo
  rules:
  - record: foo
    expr: sum(up)
    xxx: 1
    labels:
      job: foo
`,
			err:  "unexpected key `xxx`",
			line: 7,
		},
		{
			content: `
groups:
- name: foo
  rules:
    record: foo
    expr: sum(up)
    xxx: 1
    labels:
      job: foo
`,
			err:  "YAML mapping is not allowed here, expected a YAML list",
			line: 5,
		},
		{
			content: `
groups:
- name: foo
  rules:
  - record: foo
    expr: sum(up)
    labels:
      job:
        foo: bar
`,
			err:  "YAML mapping is not allowed here, expected a YAML scalar value",
			line: 9,
		},
		{
			content: `
groups:
- name: foo
  rules:
  - record: foo
    expr:
      sum: sum(up)
`,
			err:  "YAML mapping is not allowed here, expected a YAML scalar value",
			line: 7,
		},
		{
			content: `
groups:
- name: foo
  rules: []
- name: foo
  rules: []
`,
			err:  "duplicated group name `name`",
			line: 5,
		},
		{
			content: `
groups:
- name: foo
  rules:
  - record: foo
    expr: sum(up)
    labels:
      foo: bob
      foo: bar
`,
			err:  "duplicated key `foo`",
			line: 9,
		},
		{
			content: `
groups:
- name: v2
  rules:
  - record: up:count
    expr: count(up)
    expr: sum(up)`,
			err:  "duplicated key `expr`",
			line: 7,
		},
		{
			content: `
groups:
- name: v2
  rules:
  - record: up:count
    expr: count(up)
bogus: 1
`,
			err:  "unexpected key `bogus`",
			line: 7,
		},
		{
			content: `
groups:
- name: v2
  rules:
  - record: up:count
    expr: count(up)
    bogus: 1
`,
			err:  "unexpected key `bogus`",
			line: 7,
		},
		{
			content: `
groups:

- name: CloudflareKafkaZookeeperExporter

  rules:
`,
			err:  "YAML scalar value is not allowed here, expected a YAML list",
			line: 6,
		},
		{
			content: `
groups:
- name: foo
  rules:
    expr: 1
`,
			err:  "YAML mapping is not allowed here, expected a YAML list",
			line: 5,
		},
		{
			content: `
groups:
- name: foo
  rules:
    - expr: 1
`,
			err:  "expected a YAML string here, got integer instead",
			line: 5,
		},
		{
			content: `
groups:
- name: foo
  rules:
    - expr: null
`,
			err:  "expected a YAML string here, got null instead",
			line: 5,
		},
		{
			content: `
groups:
- name: foo
  rules:
    - 1: null
`,
			err:  "expected a YAML string here, got integer instead",
			line: 5,
		},
		{
			content: `
groups:
- name: foo
  rules:
    - true: !!binary "SGVsbG8sIFdvcmxkIQ=="
`,
			err:  "expected a YAML string here, got bool instead",
			line: 5,
		},
		{
			content: `
groups:
- name: foo
  rules:
    - expr: !!binary "SGVsbG8sIFdvcmxkIQ=="
`,
			err:  "expected a YAML string here, got binary data instead",
			line: 5,
		},
		{
			content: `
groups:
- name: foo
  rules:
    - labels: !!binary "SGVsbG8sIFdvcmxkIQ=="
`,
			err:  "YAML scalar value is not allowed here, expected a YAML mapping",
			line: 5,
		},
		{
			content: `
---
groups:
- name: foo
  rules:
    - record: foo
      expr: bar
---
groups:
- name: foo
  rules:
    - record: foo
      expr: bar
`,
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			t.Logf("\n--- Content ---%s--- END ---", tc.content)

			dec := yaml.NewDecoder(strings.NewReader(tc.content))
			for {
				var doc yaml.Node
				decodeErr := dec.Decode(&doc)
				if decodeErr != nil {
					require.ErrorIs(t, decodeErr, io.EOF)
					break
				}

				err := validateRuleFile(&doc)
				if tc.err == "" {
					require.NoError(t, err)
				} else {
					require.Error(t, err)
					var se StrictError
					require.ErrorAs(t, err, &se)
					require.EqualError(t, se.Err, tc.err)
					require.Equal(t, tc.line, se.Line)
				}
			}
		})
	}
}
