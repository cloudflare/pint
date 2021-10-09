package checks_test

import (
	"testing"

	"github.com/cloudflare/pint/internal/checks"
)

func TestValueCheck(t *testing.T) {
	testCases := []checkTest{
		{
			description: "ignore recording rules",
			content:     "- record: foo\n  expr: sum(foo)\n",
			checker:     checks.NewValueCheck(checks.Bug),
		},
		{
			description: "ignore alerting rules with no labels",
			content:     "- alert: foo\n  expr: sum(foo)\n",
			checker:     checks.NewValueCheck(checks.Bug),
		},
		{
			description: "static labels",
			content:     "- alert: foo\n  expr: sum(foo)\n  labels:\n    foo: bar\n    baz: bar\n",
			checker:     checks.NewValueCheck(checks.Bug),
		},
		{
			description: "{{ $value}} in label key",
			content:     "- alert: foo\n  expr: sum(foo)\n  labels:\n    foo: bar\n    '{{ $value}}': bar\n",
			checker:     checks.NewValueCheck(checks.Bug),
			problems: []checks.Problem{
				{
					Fragment: "{{ $value}}",
					Lines:    []int{5},
					Reporter: "alerts/value",
					Text:     "using $value in labels will generate a new alert on every value change, move it to annotations",
					Severity: checks.Bug,
				},
			},
		},
		{
			description: "{{ $value }} in label key",
			content:     "- alert: foo\n  expr: sum(foo)\n  labels:\n    foo: bar\n    '{{ $value }}': bar\n",
			checker:     checks.NewValueCheck(checks.Bug),
			problems: []checks.Problem{
				{
					Fragment: "{{ $value }}",
					Lines:    []int{5},
					Reporter: "alerts/value",
					Text:     "using $value in labels will generate a new alert on every value change, move it to annotations",
					Severity: checks.Bug,
				},
			},
		},
		{
			description: "{{$value}} in label value",
			content:     "- alert: foo\n  expr: sum(foo)\n  labels:\n    foo: bar\n    baz: '{{$value}}'\n",
			checker:     checks.NewValueCheck(checks.Fatal),
			problems: []checks.Problem{
				{
					Fragment: "{{$value}}",
					Lines:    []int{5},
					Reporter: "alerts/value",
					Text:     "using $value in labels will generate a new alert on every value change, move it to annotations",
					Severity: checks.Fatal,
				},
			},
		},
		{
			description: "{{  $value  }} in label value",
			content:     "- alert: foo\n  expr: sum(foo)\n  labels:\n    foo: bar\n    baz: |\n      foo is {{  $value | humanizePercentage }}%\n",
			checker:     checks.NewValueCheck(checks.Bug),
			problems: []checks.Problem{
				{
					Fragment: "foo is {{  $value | humanizePercentage }}%\n",
					Lines:    []int{5, 6},
					Reporter: "alerts/value",
					Text:     "using $value in labels will generate a new alert on every value change, move it to annotations",
					Severity: checks.Bug,
				},
			},
		},
		{
			description: "{{  $value  }} in label value",
			content:     "- alert: foo\n  expr: sum(foo)\n  labels:\n    foo: bar\n    baz: |\n      foo is {{$value|humanizePercentage}}%\n",
			checker:     checks.NewValueCheck(checks.Bug),
			problems: []checks.Problem{
				{
					Fragment: "foo is {{$value|humanizePercentage}}%\n",
					Lines:    []int{5, 6},
					Reporter: "alerts/value",
					Text:     "using $value in labels will generate a new alert on every value change, move it to annotations",
					Severity: checks.Bug,
				},
			},
		},
		{
			description: "{{ .Value }} in label value",
			content:     "- alert: foo\n  expr: sum(foo)\n  labels:\n    foo: bar\n    baz: 'value {{ .Value }}'\n",
			checker:     checks.NewValueCheck(checks.Bug),
			problems: []checks.Problem{
				{
					Fragment: "value {{ .Value }}",
					Lines:    []int{5},
					Reporter: "alerts/value",
					Text:     "using .Value in labels will generate a new alert on every value change, move it to annotations",
					Severity: checks.Bug,
				},
			},
		},
		{
			description: "{{ .Value|humanize }} in label value",
			content:     "- alert: foo\n  expr: sum(foo)\n  labels:\n    foo: bar\n    baz: '{{ .Value|humanize }}'\n",
			checker:     checks.NewValueCheck(checks.Bug),
			problems: []checks.Problem{
				{
					Fragment: "{{ .Value|humanize }}",
					Lines:    []int{5},
					Reporter: "alerts/value",
					Text:     "using .Value in labels will generate a new alert on every value change, move it to annotations",
					Severity: checks.Bug,
				},
			},
		},
	}
	runTests(t, testCases)
}
