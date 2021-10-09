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
		/*
			{
				description: "annotation label missing from metrics",
				content:     "- alert: Foo Is Down\n  expr: up{job=\"foo\"} == 0\n  annotations:\n    summary: '{{ $labels.job }}'\n",
				checker:     checks.NewTemplateCheck(checks.Bug),
				problems: []checks.Problem{
					{
						Fragment: `summary: '{{ $labels.job }}`,
						Lines:    []int{2, 3},
						Reporter: "alerts/count",
						Text:     "query using prom would trigger 1 alert(s) in the last 1d",
						Severity: checks.Information,
					},
				},
			},
		*/
	}
	runTests(t, testCases)
}
