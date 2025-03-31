package parser

import (
	"bytes"
	"io"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/comments"
)

func TestReadContent(t *testing.T) {
	type testCaseT struct {
		input       []byte
		output      []byte
		comments    []comments.Comment
		ignored     bool
		shouldError bool
	}

	testCases := []testCaseT{
		{
			input:  []byte(""),
			output: []byte(""),
		},
		{
			input:  []byte("\n"),
			output: []byte("\n"),
		},
		{
			input:  []byte("\n \n"),
			output: []byte("\n \n"),
		},
		{
			input:  []byte("foo bar"),
			output: []byte("foo bar"),
		},
		{
			input:  []byte("foo bar\n"),
			output: []byte("foo bar\n"),
		},
		{
			input:  []byte("line1\nline2"),
			output: []byte("line1\nline2"),
		},
		{
			input:  []byte("line1\nline2\n"),
			output: []byte("line1\nline2\n"),
		},
		{
			input:  []byte("line1\n\nline2\n\n"),
			output: []byte("line1\n\nline2\n\n"),
		},
		{
			input:  []byte("# pint ignore/next-line\nfoo\n"),
			output: []byte("# pint ignore/next-line\n   \n"),
		},
		{
			input:  []byte("# pint ignore/next-line\nfoo"),
			output: []byte("# pint ignore/next-line\n   "),
		},
		{
			input:  []byte("# pint ignore/next-line\nfoo\n\n"),
			output: []byte("# pint ignore/next-line\n   \n\n"),
		},
		{
			input:  []byte("# pint ignore/next-line\nfoo\nbar\n"),
			output: []byte("# pint ignore/next-line\n   \nbar\n"),
		},
		{
			input:  []byte("# pint ignore/next-line  \nfoo\n"),
			output: []byte("# pint ignore/next-line  \n   \n"),
		},
		{
			input:  []byte("#  pint   ignore/next-line  \nfoo\n"),
			output: []byte("#  pint   ignore/next-line  \n   \n"),
		},
		{
			input:  []byte("# pint   ignore/next-line  \nfoo\n"),
			output: []byte("# pint   ignore/next-line  \n   \n"),
		},
		{
			input:  []byte("# pintignore/next-line\nfoo\n"),
			output: []byte("# pintignore/next-line\nfoo\n"),
		},
		{
			input:  []byte("# pint ignore/next-linex\nfoo\n"),
			output: []byte("# pint ignore/next-linex\nfoo\n"),
		},
		{
			input:  []byte("# pint ignore/begin\nfoo\nbar\n"),
			output: []byte("# pint ignore/begin\n   \n   \n"),
		},
		{
			input:  []byte("prefix # pint ignore/begin\nfoo\nbar\n"),
			output: []byte("prefix # pint ignore/begin\n   \n   \n"),
		},
		{
			input:  []byte("# pint ignore/begin\nfoo\nbar\n# pint ignore/begin"),
			output: []byte("# pint ignore/begin\n   \n   \n# pint ignore/begin"),
		},
		{
			input:  []byte("# pint ignore/begin\nfoo\nbar\n# pint ignore/begin\nfoo\n"),
			output: []byte("# pint ignore/begin\n   \n   \n# pint ignore/begin\n   \n"),
		},
		{
			input:  []byte("# pint ignore/begin\nfoo\nbar\n# pint ignore/end\nfoo\n"),
			output: []byte("# pint ignore/begin\n   \n   \n# pint ignore/end\nfoo\n"),
		},
		{
			input:  []byte("# pint ignore/begin\nfoo # pint ignore/line\nbar\n# pint ignore/begin"),
			output: []byte("# pint ignore/begin\n                      \n   \n# pint ignore/begin"),
		},
		{
			input:  []byte("line1\nline2 # pint ignore/line\n"),
			output: []byte("line1\n      # pint ignore/line\n"),
		},
		{
			input:  []byte("line1\nline2 # pint ignore/line\nline3\n"),
			output: []byte("line1\n      # pint ignore/line\nline3\n"),
		},
		{
			input:  []byte("{#- comment #} # pint ignore/line\n"),
			output: []byte("               # pint ignore/line\n"),
		},
		{
			input:   []byte("# pint ignore/file\nfoo\nbar\n# pint ignore/begin\nfoo\n# pint ignore/end\n"),
			output:  []byte("# pint ignore/file\n   \n   \n# pint ignore/begin\n   \n# pint ignore/end\n"),
			ignored: true,
		},
		{
			input:   []byte("foo\n# pint ignore/file\nfoo\nbar\n# pint ignore/begin\nfoo\n# pint ignore/end\n"),
			output:  []byte("foo\n# pint ignore/file\n   \n   \n# pint ignore/begin\n   \n# pint ignore/end\n"),
			ignored: true,
		},
		{
			input:  []byte("  {% raw %} # pint ignore/line\n"),
			output: []byte("            # pint ignore/line\n"),
		},
		{
			input:  []byte("{# comment #} # pint ignore/line\n"),
			output: []byte("              # pint ignore/line\n"),
		},
		{
			input:  []byte("# pint file/owner bob\n# pint rule/set xxx\n# pint bamboozle xxx\n"),
			output: []byte("# pint file/owner bob\n# pint rule/set xxx\n# pint bamboozle xxx\n"),
			comments: []comments.Comment{
				{
					Type:  comments.FileOwnerType,
					Value: comments.Owner{Name: "bob", Line: 1},
				},
			},
		},
		{
			input:  []byte("{#- hide this comment -#} # pint ignore/line\n"),
			output: []byte("                          # pint ignore/line\n"),
		},
		{
			input:  []byte("# pint ignore/begin\n  - alert: Ignored\n    # pint rule/set foo\n    # pint rule/set bar\n    expr: up\n# pint ignore/end\n"),
			output: []byte("# pint ignore/begin\n                  \n                       \n                       \n            \n# pint ignore/end\n"),
		},
		{
			input: []byte(`
# pint ignore/begin
{%- set foo = 1 %}
{% set bar = 2 -%}
{# comment #}
{#
  comment 
#}
# pint ignore/end

- record: colo_job:up:count
  expr: sum(foo) without(job)

- record: invalid
  expr: sum(foo) by ())

# pint ignore/begin
- record: colo_job:down:count
  expr: up == {{ foo }}
# pint ignore/end

- record: colo:multiline
  expr: |
    sum(
      multiline
    ) without(job, instance)

- record: colo:multiline:sum
  expr: |
    sum(sum) without(job)
    +
    sum(sum) without(job)

- record: colo:multiline2
  expr: >-
    sum(
      multiline2
    ) without(job, instance)

- record: colo_job:up:byinstance
  expr: sum(byinstance) by(instance)

- record: instance_mode:node_cpu:rate4m
  expr:  sum(rate(node_cpu_seconds_total[4m])) without (cpu)

- record: instance_mode:node_cpu:rate4m
  expr:  sum(rate(node_cpu_seconds_total[5m])) without (cpu)

- record: instance_mode:node_cpu:rate5min
  expr:  sum(irate(node_cpu_seconds_total[5m])) without (cpu)

- alert: Instance Is Down
  expr: up == 0

- alert: Error Rate
  expr: sum(rate(errors[5m])) > 0.5

- alert: Error Rate
  expr: sum(rate(errors[5m])) > 0.5
  annotations:
    link: http://docs
    summary: 'error rate: {{ $value }}'
`),
			output: []byte(`
# pint ignore/begin
                  
                  
             
  
          
  
# pint ignore/end

- record: colo_job:up:count
  expr: sum(foo) without(job)

- record: invalid
  expr: sum(foo) by ())

# pint ignore/begin
                             
                       
# pint ignore/end

- record: colo:multiline
  expr: |
    sum(
      multiline
    ) without(job, instance)

- record: colo:multiline:sum
  expr: |
    sum(sum) without(job)
    +
    sum(sum) without(job)

- record: colo:multiline2
  expr: >-
    sum(
      multiline2
    ) without(job, instance)

- record: colo_job:up:byinstance
  expr: sum(byinstance) by(instance)

- record: instance_mode:node_cpu:rate4m
  expr:  sum(rate(node_cpu_seconds_total[4m])) without (cpu)

- record: instance_mode:node_cpu:rate4m
  expr:  sum(rate(node_cpu_seconds_total[5m])) without (cpu)

- record: instance_mode:node_cpu:rate5min
  expr:  sum(irate(node_cpu_seconds_total[5m])) without (cpu)

- alert: Instance Is Down
  expr: up == 0

- alert: Error Rate
  expr: sum(rate(errors[5m])) > 0.5

- alert: Error Rate
  expr: sum(rate(errors[5m])) > 0.5
  annotations:
    link: http://docs
    summary: 'error rate: {{ $value }}'
`),
		},
	}

	cmpErrorText := cmp.Comparer(func(x, y any) bool {
		xe := x.(error)
		ye := y.(error)
		return xe.Error() == ye.Error()
	})
	sameErrorText := cmp.FilterValues(func(x, y any) bool {
		_, xe := x.(error)
		_, ye := y.(error)
		return xe && ye
	}, cmpErrorText)

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			r := newContentReader(bytes.NewReader(tc.input))
			output, err := io.ReadAll(r)

			hadError := err != nil
			if hadError != tc.shouldError {
				t.Errorf("ReadContent() returned err=%v, expected=%v", err, tc.shouldError)
				return
			}

			if diff := cmp.Diff(tc.comments, r.Comments(), sameErrorText); diff != "" {
				t.Errorf("ReadContent() returned wrong comments (-want +got):\n%s", diff)
				return
			}

			require.Equal(t, string(tc.output), string(output), "ReadContent() returned wrong output")
			require.Equal(t, tc.ignored, r.Ignored(), "ReadContent() returned wrong Ignored value")
		})
	}
}
