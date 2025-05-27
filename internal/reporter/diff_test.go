package reporter

import (
	"strconv"
	"strings"
	"testing"

	"github.com/akedrou/textdiff"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestParseDiffLines(t *testing.T) {
	type testCaseT struct {
		old   string
		new   string
		lines []diffLine
	}

	testCases := []testCaseT{
		{
			old:   "",
			new:   "",
			lines: nil,
		},
		{
			old: "a\n",
			new: "b\n",
			lines: []diffLine{
				{old: 1, new: 1, wasModified: true},
			},
		},
		{
			old: "a\nb\nc\n",
			new: "a\nc\n",
			lines: []diffLine{
				{old: 2, new: 1, wasModified: true},
				{old: 3, new: 2},
			},
		},
		{
			old: "a\nb\nc\n",
			new: "a\nd\nc\n",
			lines: []diffLine{
				{old: 1, new: 1},
				{old: 2, new: 2, wasModified: true},
				{old: 3, new: 3},
			},
		},
		{
			old: `
- record: foo
  expr: |
    sum(foo) by(cluster)
  labels: {}
`,
			new: `
- record: foo
  expr: sum(foo) by(cluster)
  labels:
    env: prod
`,
			lines: []diffLine{
				{old: 1, new: 1},
				{old: 2, new: 2},
				{old: 4, new: 3, wasModified: true},
				{old: 5, new: 4, wasModified: true},
				{old: 5, new: 5, wasModified: true},
			},
		},
		{
			old: `
- record: foo
  expr: |
    sum(foo) by(cluster)
  labels:
`,
			new: `
- record: foo
  expr: |
    sum(foo) by(cluster)
  labels:
    env: prod
`,
			lines: []diffLine{
				{old: 3, new: 3},
				{old: 4, new: 4},
				{old: 5, new: 5},
				{old: 5, new: 6, wasModified: true},
			},
		},
		{
			old: `
- record: foo
  expr: |
    sum(foo) by(cluster)
  labels: {}
`,
			new: `
- record: foo
  expr: |
    sum(foo) by(cluster)
  labels:
    env: prod
`,
			lines: []diffLine{
				{old: 2, new: 2},
				{old: 3, new: 3},
				{old: 4, new: 4},
				{old: 5, new: 5, wasModified: true},
				{old: 5, new: 6, wasModified: true},
			},
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			uni := textdiff.Unified("old.txt", "new.txt", tc.old, tc.new)
			gl := uni
			if strings.Count(gl, "\n") > 2 {
				gl = strings.Join(strings.Split(uni, "\n")[2:], "\n")
			}
			for name, diff := range map[string]string{
				"unified": uni,
				"gitlab":  gl,
			} {
				t.Run(name, func(t *testing.T) {
					t.Logf("Diff: %s", diff)
					lines := parseDiffLines(diff)
					if diff := cmp.Diff(tc.lines, lines, cmpopts.EquateComparable(diffLine{})); diff != "" {
						t.Errorf("Wrong parseDiffLines() output: (-want +got):\n%s", diff)
						t.FailNow()
					}
				})
			}
		})
	}
}
