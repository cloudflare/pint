package diags

import (
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

func parseYaml(input string) (key, val *yaml.Node) {
	dec := yaml.NewDecoder(strings.NewReader(input))
	for {
		var doc yaml.Node
		err := dec.Decode(&doc)
		if errors.Is(err, io.EOF) {
			break
		}
		for _, root := range doc.Content {
			if root.Kind == yaml.MappingNode {
				key = root.Content[0]
				val = root.Content[1]
			}
		}
	}
	return key, val
}

func TestNewPositions(t *testing.T) {
	type testCaseT struct {
		input  string
		output PositionRanges
	}

	testCases := []testCaseT{
		{
			input: `foo: my very long string`,
			output: PositionRanges{
				{Line: 1, FirstColumn: 6, LastColumn: 24},
			},
		},
		{
			input: `

foo: my
  very long
  string
`,
			output: PositionRanges{
				{Line: 3, FirstColumn: 6, LastColumn: 8},
				{Line: 4, FirstColumn: 3, LastColumn: 12},
				{Line: 5, FirstColumn: 3, LastColumn: 8},
			},
		},
		{
			input: `
foo: |
  my
  very long
  string
`,
			output: PositionRanges{
				{Line: 3, FirstColumn: 3, LastColumn: 5},
				{Line: 4, FirstColumn: 3, LastColumn: 12},
				{Line: 5, FirstColumn: 3, LastColumn: 8},
			},
		},
		{
			input: `
expr:
  sum(bar{job="foo"})
  / on(c,d)
  sum(bar)
`,
			output: PositionRanges{
				{Line: 3, FirstColumn: 3, LastColumn: 22},
				{Line: 4, FirstColumn: 3, LastColumn: 12},
				{Line: 5, FirstColumn: 3, LastColumn: 10},
			},
		},
		{
			input: `
for: |+
  11m
bar: xxx
`,
			output: PositionRanges{
				{Line: 3, FirstColumn: 3, LastColumn: 5},
			},
		},
		{
			input: `
expr:
  (
    xxx
    -
    yyy
  ) * bar > 0
  and on(instance, device) baz
`,
			output: PositionRanges{
				{Line: 3, FirstColumn: 3, LastColumn: 4},
				{Line: 4, FirstColumn: 5, LastColumn: 8},
				{Line: 5, FirstColumn: 5, LastColumn: 6},
				{Line: 6, FirstColumn: 5, LastColumn: 8},
				{Line: 7, FirstColumn: 3, LastColumn: 14},
				{Line: 8, FirstColumn: 3, LastColumn: 30},
			},
		},
		{
			input: `
expr: |
    sum without (name) (
        bird_protocol_prefix_export_count{ip_version="4",name=~".*external.*",proto!="Kernel"}
      * on (instance) group_left (profile,cluster)
        cf_node_role{kubernetes_role="director",role="kubernetes"}
    ) <= 0
`,
			output: PositionRanges{
				{Line: 3, FirstColumn: 5, LastColumn: 25},
				{Line: 4, FirstColumn: 5, LastColumn: 95},
				{Line: 5, FirstColumn: 5, LastColumn: 51},
				{Line: 6, FirstColumn: 5, LastColumn: 67},
				{Line: 7, FirstColumn: 5, LastColumn: 10},
			},
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			key, val := parseYaml(tc.input)
			t.Logf("KEY [%s] %+v", key.Value, key)
			t.Logf("VAL [%s] %+v", val.Value, val)
			output := NewPositionRange(strings.Split(tc.input, "\n"), val, key.Column+2)
			if diff := cmp.Diff(tc.output, output); diff != "" {
				t.Errorf("NewPositions() returned wrong output (-want +got):\n%s", diff)
				return
			}

			require.Equal(t, len(strings.TrimSuffix(val.Value, "\n")), output.Len())
		})
	}
}

func TestReadRange(t *testing.T) {
	type testCaseT struct {
		input  string
		output PositionRanges
		first  int
		last   int
	}

	testCases := []testCaseT{
		{
			input: `

foo: my
  very long
  string
`,
			first: 4,
			last:  7,
			output: PositionRanges{
				{Line: 4, FirstColumn: 3, LastColumn: 6},
			},
		},
		{
			input: `
expr: |
  sum(bar{job="foo"})
  / on(c,d)
  sum(bar)
`,
			first: 23,
			last:  24,
			output: PositionRanges{
				{Line: 4, FirstColumn: 5, LastColumn: 6},
			},
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			key, val := parseYaml(tc.input)
			pos := NewPositionRange(strings.Split(tc.input, "\n"), val, key.Column+2)
			output := readRange(tc.first, tc.last, pos)
			if diff := cmp.Diff(tc.output, output); diff != "" {
				t.Errorf("ReadRange() returned wrong output (-want +got):\n%s", diff)
				return
			}
		})
	}
}

func TestLineRangeString(t *testing.T) {
	type testCaseT struct {
		expected string
		lr       LineRange
	}

	testCases := []testCaseT{
		{lr: LineRange{First: 1, Last: 1}, expected: "1"},
		{lr: LineRange{First: 1, Last: 2}, expected: "1-2"},
	}

	for _, tc := range testCases {
		t.Run(tc.expected, func(t *testing.T) {
			require.Equal(t, tc.expected, tc.lr.String())
		})
	}
}

func TestLineRangeExpand(t *testing.T) {
	lr := LineRange{First: 1, Last: 3}
	require.Equal(t, []int{1, 2, 3}, lr.Expand())
}

func TestPositionRangesLines(t *testing.T) {
	prs := PositionRanges{
		{Line: 2},
		{Line: 5},
		{Line: 3},
	}
	lr := prs.Lines()
	require.Equal(t, 2, lr.First)
	require.Equal(t, 5, lr.Last)
}

func TestPositionRangesAddOffset(t *testing.T) {
	prs := PositionRanges{
		{Line: 1, FirstColumn: 2, LastColumn: 3},
		{Line: 2, FirstColumn: 3, LastColumn: 4},
	}
	prs.AddOffset(10, 20)
	require.Equal(t, 11, prs[0].Line)
	require.Equal(t, 22, prs[0].FirstColumn)
	require.Equal(t, 23, prs[0].LastColumn)
	require.Equal(t, 12, prs[1].Line)
	require.Equal(t, 23, prs[1].FirstColumn)
	require.Equal(t, 24, prs[1].LastColumn)
}
