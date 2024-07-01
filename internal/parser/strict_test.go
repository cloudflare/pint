package parser

import (
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestValidateRuleFile(t *testing.T) {
	type testCaseT struct {
		content string
		errs    []ParseError
	}

	testCases := []testCaseT{
		{
			content: "[]",
			errs: []ParseError{
				{
					Err:  errors.New("YAML list is not allowed here, expected a YAML mapping"),
					Line: 1,
				},
			},
		},
		{
			content: "\n\n[]",
			errs: []ParseError{
				{
					Err:  errors.New("YAML list is not allowed here, expected a YAML mapping"),
					Line: 3,
				},
			},
		},
		{
			content: "groups: {}",
			errs: []ParseError{
				{
					Err:  errors.New("YAML mapping is not allowed here, expected a YAML list"),
					Line: 1,
				},
			},
		},
		{
			content: "groups: []",
		},
		{
			content: "xgroups: {}",
			errs: []ParseError{
				{
					Err:  errors.New("unexpected key `xgroups`"),
					Line: 1,
				},
			},
		},
		{
			content: "\nbob\n",
			errs: []ParseError{
				{
					Err:  errors.New("YAML scalar value is not allowed here, expected a YAML mapping"),
					Line: 2,
				},
			},
		},
		{
			content: `groups: []

rules: []
`,
			errs: []ParseError{
				{
					Err:  errors.New("unexpected key `rules`"),
					Line: 3,
				},
			},
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
			errs: []ParseError{
				{
					Err:  errors.New("expected a YAML string here, got null instead"),
					Line: 3,
				},
			},
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
			errs: []ParseError{
				{
					Err:  errors.New("unexpected key `xxx`"),
					Line: 7,
				},
			},
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
			errs: []ParseError{
				{
					Err:  errors.New("YAML mapping is not allowed here, expected a YAML list"),
					Line: 5,
				},
			},
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
			errs: []ParseError{
				{
					Err:  errors.New("YAML mapping is not allowed here, expected a YAML scalar value"),
					Line: 9,
				},
			},
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
			errs: []ParseError{
				{
					Err:  errors.New("YAML mapping is not allowed here, expected a YAML scalar value"),
					Line: 7,
				},
			},
		},
		{
			content: `
groups:
- name: foo
  rules: []
- name: foo
  rules: []
`,
			errs: []ParseError{
				{
					Err:  errors.New("duplicated group name `foo`"),
					Line: 5,
				},
			},
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
			errs: []ParseError{
				{
					Err:  errors.New("duplicated key `foo`"),
					Line: 9,
				},
			},
		},
		{
			content: `
groups:
- name: v2
  rules:
  - record: up:count
    expr: count(up)
    expr: sum(up)`,
			errs: []ParseError{
				{
					Err:  errors.New("duplicated key `expr`"),
					Line: 7,
				},
			},
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
			errs: []ParseError{
				{
					Err:  errors.New("unexpected key `bogus`"),
					Line: 7,
				},
			},
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
			errs: []ParseError{
				{
					Err:  errors.New("unexpected key `bogus`"),
					Line: 7,
				},
			},
		},
		{
			content: `
groups:

- name: CloudflareKafkaZookeeperExporter

  rules:
`,
			errs: []ParseError{
				{
					Err:  errors.New("YAML scalar value is not allowed here, expected a YAML list"),
					Line: 6,
				},
			},
		},
		{
			content: `
groups:
- name: foo
  rules:
    expr: 1
`,
			errs: []ParseError{
				{
					Err:  errors.New("YAML mapping is not allowed here, expected a YAML list"),
					Line: 5,
				},
			},
		},
		{
			content: `
groups:
- name: foo
  rules:
    - expr: 1
`,
			errs: []ParseError{
				{
					Err:  errors.New("expected a YAML string here, got integer instead"),
					Line: 5,
				},
			},
		},
		{
			content: `
groups:
- name: foo
  rules:
    - expr: null
`,
			errs: []ParseError{
				{
					Err:  errors.New("expected a YAML string here, got null instead"),
					Line: 5,
				},
			},
		},
		{
			content: `
groups:
- name: foo
  rules:
    - 1: null
`,
			errs: []ParseError{
				{
					Err:  errors.New("expected a YAML string here, got integer instead"),
					Line: 5,
				},
			},
		},
		{
			content: `
groups:
- name: foo
  rules:
    - true: !!binary "SGVsbG8sIFdvcmxkIQ=="
`,
			errs: []ParseError{
				{
					Err:  errors.New("expected a YAML string here, got bool instead"),
					Line: 5,
				},
			},
		},
		{
			content: `
groups:
- name: foo
  rules:
    - expr: !!binary "SGVsbG8sIFdvcmxkIQ=="
`,
			errs: []ParseError{
				{
					Err:  errors.New("expected a YAML string here, got binary data instead"),
					Line: 5,
				},
			},
		},
		{
			content: `
groups:
- name: foo
  rules:
    - labels: !!binary "SGVsbG8sIFdvcmxkIQ=="
`,
			errs: []ParseError{
				{
					Err:  errors.New("YAML scalar value is not allowed here, expected a YAML mapping"),
					Line: 5,
				},
			},
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

	cmpErrorText := cmp.Comparer(func(x, y interface{}) bool {
		xe := x.(error)
		ye := y.(error)
		return xe.Error() == ye.Error()
	})
	sameErrorText := cmp.FilterValues(func(x, y interface{}) bool {
		_, xe := x.(error)
		_, ye := y.(error)
		return xe && ye
	}, cmpErrorText)

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

				errs := validateRuleFile(&doc)
				if diff := cmp.Diff(tc.errs, errs, sameErrorText); diff != "" {
					t.Errorf("validateRuleFile() returned wrong output (-want +got):\n%s", diff)
					return
				}
			}
		})
	}
}
