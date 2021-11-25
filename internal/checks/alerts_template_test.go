package checks_test

import (
	"testing"

	"github.com/cloudflare/pint/internal/checks"
)

func TestTemplateCheck(t *testing.T) {
	testCases := []checkTest{
		{
			description: "skips recording rule",
			content:     "- record: foo\n  expr: sum(foo)\n",
			checker:     checks.NewTemplateCheck(checks.Bug),
		},
		{
			description: "invalid syntax in annotations",
			content:     "- alert: Foo Is Down\n  expr: up{job=\"foo\"} == 0\n  annotations:\n    summary: 'Instance {{ $label.instance }} down'\n",
			checker:     checks.NewTemplateCheck(checks.Bug),
			problems: []checks.Problem{
				{
					Fragment: `summary: Instance {{ $label.instance }} down`,
					Lines:    []int{4},
					Reporter: "alerts/template",
					Text:     "template parse error: undefined variable \"$label\"",
					Severity: checks.Bug,
				},
			},
		},
		{
			description: "invalid function in annotations",
			content:     "- alert: Foo Is Down\n  expr: up{job=\"foo\"} == 0\n  annotations:\n    summary: '{{ $value | xxx }}'\n",
			checker:     checks.NewTemplateCheck(checks.Bug),
			problems: []checks.Problem{
				{
					Fragment: `summary: {{ $value | xxx }}`,
					Lines:    []int{4},
					Reporter: "alerts/template",
					Text:     "template parse error: function \"xxx\" not defined",
					Severity: checks.Bug,
				},
			},
		},
		{
			description: "valid syntax in annotations",
			content:     "- alert: Foo Is Down\n  expr: up{job=\"foo\"} == 0\n  annotations:\n    summary: 'Instance {{ $labels.instance }} down'\n",
			checker:     checks.NewTemplateCheck(checks.Bug),
		},
		{
			description: "invalid syntax in labels",
			content:     "- alert: Foo Is Down\n  expr: up{job=\"foo\"} == 0\n  labels:\n    summary: 'Instance {{ $label.instance }} down'\n",
			checker:     checks.NewTemplateCheck(checks.Bug),
			problems: []checks.Problem{
				{
					Fragment: `summary: Instance {{ $label.instance }} down`,
					Lines:    []int{4},
					Reporter: "alerts/template",
					Text:     "template parse error: undefined variable \"$label\"",
					Severity: checks.Bug,
				},
			},
		},
		{
			description: "invalid function in annotations",
			content:     "- alert: Foo Is Down\n  expr: up{job=\"foo\"} == 0\n  labels:\n    summary: '{{ $value | xxx }}'\n",
			checker:     checks.NewTemplateCheck(checks.Bug),
			problems: []checks.Problem{
				{
					Fragment: `summary: {{ $value | xxx }}`,
					Lines:    []int{4},
					Reporter: "alerts/template",
					Text:     "template parse error: function \"xxx\" not defined",
					Severity: checks.Bug,
				},
			},
		},
		{
			description: "valid syntax in labels",
			content:     "- alert: Foo Is Down\n  expr: up{job=\"foo\"} == 0\n  labels:\n    summary: 'Instance {{ $labels.instance }} down'\n",
			checker:     checks.NewTemplateCheck(checks.Bug),
		},
		{
			description: "{{ $value}} in label key",
			content:     "- alert: foo\n  expr: sum(foo)\n  labels:\n    foo: bar\n    '{{ $value}}': bar\n",
			checker:     checks.NewTemplateCheck(checks.Bug),
			problems: []checks.Problem{
				{
					Fragment: "{{ $value}}: bar",
					Lines:    []int{5},
					Reporter: "alerts/template",
					Text:     "using $value in labels will generate a new alert on every value change, move it to annotations",
					Severity: checks.Bug,
				},
			},
		},
		{
			description: "{{ $value }} in label key",
			content:     "- alert: foo\n  expr: sum(foo)\n  labels:\n    foo: bar\n    '{{ $value }}': bar\n",
			checker:     checks.NewTemplateCheck(checks.Bug),
			problems: []checks.Problem{
				{
					Fragment: "{{ $value }}: bar",
					Lines:    []int{5},
					Reporter: "alerts/template",
					Text:     "using $value in labels will generate a new alert on every value change, move it to annotations",
					Severity: checks.Bug,
				},
			},
		},
		{
			description: "{{$value}} in label value",
			content:     "- alert: foo\n  expr: sum(foo)\n  labels:\n    foo: bar\n    baz: '{{$value}}'\n",
			checker:     checks.NewTemplateCheck(checks.Fatal),
			problems: []checks.Problem{
				{
					Fragment: "baz: {{$value}}",
					Lines:    []int{5},
					Reporter: "alerts/template",
					Text:     "using $value in labels will generate a new alert on every value change, move it to annotations",
					Severity: checks.Fatal,
				},
			},
		},
		{
			description: "{{$value}} in multiple labels",
			content:     "- alert: foo\n  expr: sum(foo)\n  labels:\n    foo: '{{ .Value }}'\n    baz: '{{$value}}'\n",
			checker:     checks.NewTemplateCheck(checks.Fatal),
			problems: []checks.Problem{
				{
					Fragment: "foo: {{ .Value }}",
					Lines:    []int{4},
					Reporter: "alerts/template",
					Text:     "using .Value in labels will generate a new alert on every value change, move it to annotations",
					Severity: checks.Fatal,
				},
				{
					Fragment: "baz: {{$value}}",
					Lines:    []int{5},
					Reporter: "alerts/template",
					Text:     "using $value in labels will generate a new alert on every value change, move it to annotations",
					Severity: checks.Fatal,
				},
			},
		},
		{
			description: "{{  $value  }} in label value",
			content:     "- alert: foo\n  expr: sum(foo)\n  labels:\n    foo: bar\n    baz: |\n      foo is {{  $value | humanizePercentage }}%\n",
			checker:     checks.NewTemplateCheck(checks.Bug),
			problems: []checks.Problem{
				{
					Fragment: "baz: foo is {{  $value | humanizePercentage }}%\n",
					Lines:    []int{5, 6},
					Reporter: "alerts/template",
					Text:     "using $value in labels will generate a new alert on every value change, move it to annotations",
					Severity: checks.Bug,
				},
			},
		},
		{
			description: "{{  $value  }} in label value",
			content:     "- alert: foo\n  expr: sum(foo)\n  labels:\n    foo: bar\n    baz: |\n      foo is {{$value|humanizePercentage}}%\n",
			checker:     checks.NewTemplateCheck(checks.Bug),
			problems: []checks.Problem{
				{
					Fragment: "baz: foo is {{$value|humanizePercentage}}%\n",
					Lines:    []int{5, 6},
					Reporter: "alerts/template",
					Text:     "using $value in labels will generate a new alert on every value change, move it to annotations",
					Severity: checks.Bug,
				},
			},
		},
		{
			description: "{{ .Value }} in label value",
			content:     "- alert: foo\n  expr: sum(foo)\n  labels:\n    foo: bar\n    baz: 'value {{ .Value }}'\n",
			checker:     checks.NewTemplateCheck(checks.Bug),
			problems: []checks.Problem{
				{
					Fragment: "baz: value {{ .Value }}",
					Lines:    []int{5},
					Reporter: "alerts/template",
					Text:     "using .Value in labels will generate a new alert on every value change, move it to annotations",
					Severity: checks.Bug,
				},
			},
		},
		{
			description: "{{ .Value|humanize }} in label value",
			content:     "- alert: foo\n  expr: sum(foo)\n  labels:\n    foo: bar\n    baz: '{{ .Value|humanize }}'\n",
			checker:     checks.NewTemplateCheck(checks.Bug),
			problems: []checks.Problem{
				{
					Fragment: "baz: {{ .Value|humanize }}",
					Lines:    []int{5},
					Reporter: "alerts/template",
					Text:     "using .Value in labels will generate a new alert on every value change, move it to annotations",
					Severity: checks.Bug,
				},
			},
		},
		{
			description: "{{ $foo := $value }} in label value",
			content:     "- alert: foo\n  expr: sum(foo)\n  labels:\n    foo: bar\n    baz: '{{ $foo := $value }}{{ $foo }}'\n",
			checker:     checks.NewTemplateCheck(checks.Bug),
			problems: []checks.Problem{
				{
					Fragment: "baz: {{ $foo := $value }}{{ $foo }}",
					Lines:    []int{5},
					Reporter: "alerts/template",
					Text:     "using $foo in labels will generate a new alert on every value change, move it to annotations",
					Severity: checks.Bug,
				},
			},
		},
		{
			description: "{{ $foo := .Value }} in label value",
			content:     "- alert: foo\n  expr: sum(foo)\n  labels:\n    foo: bar\n    baz: '{{ $foo := .Value }}{{ $foo }}'\n",
			checker:     checks.NewTemplateCheck(checks.Bug),
			problems: []checks.Problem{
				{
					Fragment: "baz: {{ $foo := .Value }}{{ $foo }}",
					Lines:    []int{5},
					Reporter: "alerts/template",
					Text:     "using $foo in labels will generate a new alert on every value change, move it to annotations",
					Severity: checks.Bug,
				},
			},
		},
		{
			description: "annotation label missing from metrics (by)",
			content:     "- alert: Foo Is Down\n  expr: sum(foo) > 0\n  annotations:\n    summary: '{{ $labels.job }}'\n",
			checker:     checks.NewTemplateCheck(checks.Bug),
			problems: []checks.Problem{
				{
					Fragment: `summary: {{ $labels.job }}`,
					Lines:    []int{4},
					Reporter: "alerts/template",
					Text:     `template is using "job" label but the query doesn't preseve it`,
					Severity: checks.Bug,
				},
			},
		},
		{
			description: "annotation label missing from metrics (by)",
			content:     "- alert: Foo Is Down\n  expr: sum(foo) > 0\n  annotations:\n    summary: '{{ .Labels.job }}'\n",
			checker:     checks.NewTemplateCheck(checks.Bug),
			problems: []checks.Problem{
				{
					Fragment: `summary: {{ .Labels.job }}`,
					Lines:    []int{4},
					Reporter: "alerts/template",
					Text:     `template is using "job" label but the query doesn't preseve it`,
					Severity: checks.Bug,
				},
			},
		},
		{
			description: "annotation label missing from metrics (without)",
			content:     "- alert: Foo Is Down\n  expr: sum(foo) without(job) > 0\n  annotations:\n    summary: '{{ $labels.job }}'\n",
			checker:     checks.NewTemplateCheck(checks.Bug),
			problems: []checks.Problem{
				{
					Fragment: `summary: {{ $labels.job }}`,
					Lines:    []int{4},
					Reporter: "alerts/template",
					Text:     `template is using "job" label but the query doesn't preseve it`,
					Severity: checks.Bug,
				},
			},
		},
		{
			description: "annotation label missing from metrics (without)",
			content:     "- alert: Foo Is Down\n  expr: sum(foo) without(job) > 0\n  annotations:\n    summary: '{{ .Labels.job }}'\n",
			checker:     checks.NewTemplateCheck(checks.Bug),
			problems: []checks.Problem{
				{
					Fragment: `summary: {{ .Labels.job }}`,
					Lines:    []int{4},
					Reporter: "alerts/template",
					Text:     `template is using "job" label but the query doesn't preseve it`,
					Severity: checks.Bug,
				},
			},
		},
		{
			description: "annotation label missing from metrics (or)",
			content:     "- alert: Foo Is Down\n  expr: sum(foo) by(job) or sum(bar)\n  annotations:\n    summary: '{{ .Labels.job }}'\n",
			checker:     checks.NewTemplateCheck(checks.Bug),
			problems: []checks.Problem{
				{
					Fragment: `summary: {{ .Labels.job }}`,
					Lines:    []int{4},
					Reporter: "alerts/template",
					Text:     `template is using "job" label but the query doesn't preseve it`,
					Severity: checks.Bug,
				},
			},
		},
		{
			description: "annotation label missing from metrics (1+)",
			content:     "- alert: Foo Is Down\n  expr: 1 + sum(foo) by(job) + sum(foo) by(notjob)\n  annotations:\n    summary: '{{ .Labels.job }}'\n",
			checker:     checks.NewTemplateCheck(checks.Bug),
			problems: []checks.Problem{
				{
					Fragment: `summary: {{ .Labels.job }}`,
					Lines:    []int{4},
					Reporter: "alerts/template",
					Text:     `template is using "job" label but the query doesn't preseve it`,
					Severity: checks.Bug,
				},
			},
		},
	}
	runTests(t, testCases)
}
