package discovery_test

import (
	"fmt"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/discovery"
)

func blameLine(sha string, line int, filename string) string {
	return fmt.Sprintf(`%s %d %d 1
author Alice Mock
author-mail <alice@example.com>
author-time 1559927997
author-tz 0000
committer Alice Mock
committer-mail <alice@example.com>
committer-time 1559927997
committer-tz 0000
summary Mock commit title
boundary
filename %s
	fake content
`, sha, line, line, filename)
}

type blameRange struct {
	sha   string
	lines []int
}

func blame(v map[string][]blameRange) []byte {
	var out string
	for path, brs := range v {
		for _, br := range brs {
			for _, l := range br.lines {
				out += blameLine(br.sha, l, path)
			}
		}
	}
	return []byte(out)
}

type rule struct {
	path     string
	name     string
	lines    []int
	modified []int
}

func TestGitBranchFinder(t *testing.T) {
	type testCaseT struct {
		files  map[string]string
		finder discovery.GitBranchFinder
		rules  []rule
		err    string
	}

	testRuleBody := `
- record: first
  expr: sum(foo)

- alert: second
  expr: foo > bar
  labels:
    cluster: dev

- record: third
  expr: count(foo)
  labels:
    cluster: dev
`

	testCases := []testCaseT{
		{
			files: map[string]string{},
			finder: discovery.NewGitBranchFinder(
				func(args ...string) ([]byte, error) {
					return nil, fmt.Errorf("mock error")
				},
				nil,
				"main",
				0,
				nil,
			),
			err: "failed to get the list of commits to scan: mock error",
		},
		{
			files: map[string]string{},
			finder: discovery.NewGitBranchFinder(
				func(args ...string) ([]byte, error) {
					switch strings.Join(args, " ") {
					case "log --format=%H --no-abbrev-commit --reverse main..HEAD":
						return []byte("commit1\n"), nil
					default:
						return nil, fmt.Errorf("mock error")
					}
				},
				nil,
				"main",
				0,
				nil,
			),
			err: "failed to get the list of modified files from git: mock error",
		},
		{
			files: map[string]string{},
			finder: discovery.NewGitBranchFinder(
				func(args ...string) ([]byte, error) {
					switch strings.Join(args, " ") {
					case "log --format=%H --no-abbrev-commit --reverse main..HEAD":
						return []byte("commit1\n"), nil
					case "log --reverse --no-merges --pretty=format:%H --name-status commit1^..commit1":
						return []byte("commit1\nM       foo.yml\n"), nil
					case "show -s --format=%B commit1":
						return nil, fmt.Errorf("mock error")
					default:
						t.Errorf("unknown args: %v", args)
						t.FailNow()
						return nil, nil
					}
				},
				nil,
				"main",
				0,
				[]*regexp.Regexp{regexp.MustCompile(".*")},
			),
			err: "failed to get commit message for commit1: mock error",
		},
		{
			files: map[string]string{},
			finder: discovery.NewGitBranchFinder(
				func(args ...string) ([]byte, error) {
					switch strings.Join(args, " ") {
					case "log --format=%H --no-abbrev-commit --reverse main..HEAD":
						return []byte("commit1\n"), nil
					case "log --reverse --no-merges --pretty=format:%H --name-status commit1^..commit1":
						return []byte("commit1\nM       foo.yml\n"), nil
					case "show -s --format=%B commit1":
						return []byte("foo"), nil
					case "blame --line-porcelain -- foo.yml":
						return nil, fmt.Errorf("mock error")
					default:
						t.Errorf("unknown args: %v", args)
						t.FailNow()
						return nil, nil
					}
				},
				nil,
				"main",
				0,
				[]*regexp.Regexp{regexp.MustCompile(".*")},
			),
			err: "failed to run git blame for foo.yml: mock error",
		},
		{
			files: map[string]string{},
			finder: discovery.NewGitBranchFinder(
				func(args ...string) ([]byte, error) {
					switch strings.Join(args, " ") {
					case "log --format=%H --no-abbrev-commit --reverse main..HEAD":
						return []byte("commit1\n"), nil
					case "log --reverse --no-merges --pretty=format:%H --name-status commit1^..commit1":
						return []byte("commit1\nM       foo.yml\n"), nil
					case "show -s --format=%B commit1":
						return []byte("foo"), nil
					case "blame --line-porcelain -- foo.yml":
						return blame(map[string][]blameRange{
							"foo.yml": {
								{sha: "commitX", lines: []int{1, 3, 4, 5, 6, 9, 10, 11, 12}},
								{sha: "commit1", lines: []int{2, 7, 8}},
							},
						}), nil
					default:
						t.Errorf("unknown args: %v", args)
						t.FailNow()
						return nil, nil
					}
				},
				nil,
				"main",
				0,
				[]*regexp.Regexp{regexp.MustCompile(".*")},
			),
			err: "open foo.yml: no such file or directory",
		},
		{
			files: map[string]string{
				"foo.yml": testRuleBody,
			},
			finder: discovery.NewGitBranchFinder(
				func(args ...string) ([]byte, error) {
					switch strings.Join(args, " ") {
					case "log --format=%H --no-abbrev-commit --reverse main..HEAD":
						return []byte("commit1\n"), nil
					case "log --reverse --no-merges --pretty=format:%H --name-status commit1^..commit1":
						return []byte("commit1\nM       foo.yml\n"), nil
					case "show -s --format=%B commit1":
						return []byte("foo"), nil
					case "blame --line-porcelain -- foo.yml":
						return blame(map[string][]blameRange{
							"foo.yml": {
								{sha: "commitX", lines: []int{1, 3, 4, 5, 6, 9, 10, 11, 12}},
								{sha: "commit1", lines: []int{2, 7, 8}},
							},
						}), nil
					default:
						t.Errorf("unknown args: %v", args)
						t.FailNow()
						return nil, nil
					}
				},
				nil,
				"main",
				0,
				[]*regexp.Regexp{regexp.MustCompile(".*")},
			),
			rules: []rule{
				{path: "foo.yml", name: "first", lines: []int{2, 3}, modified: []int{2}},
				{path: "foo.yml", name: "second", lines: []int{5, 6, 7, 8}, modified: []int{7, 8}},
			},
		},
		{
			files: map[string]string{
				"foo/i1.yml":  testRuleBody,
				"i2.yml":      testRuleBody,
				"foo/c1a.yml": testRuleBody,
				"foo/c1b.yml": testRuleBody,
				"c2a.yml":     testRuleBody,
				"c2c.yml":     testRuleBody,
				"foo/c2b.yml": testRuleBody,
				"bar/c3a.yml": testRuleBody,
				"c3b.yml":     testRuleBody,
				"c3c.yml":     testRuleBody,
				"c3d.yml":     testRuleBody,
				"c3e.yml":     testRuleBody,
			},
			finder: discovery.NewGitBranchFinder(
				func(args ...string) ([]byte, error) {
					switch strings.Join(args, " ") {
					case "log --format=%H --no-abbrev-commit --reverse main..HEAD":
						return []byte("commit1\ncommit2\ncommit3\n"), nil
					case "log --reverse --no-merges --pretty=format:%H --name-status commit1^..commit3":
						return []byte(`commit1
M       foo/c1a.yml
M       foo/c1b.yml
commit2
M       c2a.yml
M       foo/c2b.yml
A       foo/c2c.yml
commit3

A       bar/c3a.yml
R053    src.txt        c3b.yml
R100    foo/c3c.txt        c3c.yml
M       c2a.yml
C50     foo/cp1.yml         c3d.yml
D       foo/c2b.yml
R090    foo/c2c.yml         c2c.yml
`), nil
					case "show -s --format=%B commit1":
						return []byte("foo"), nil
					case "show -s --format=%B commit2":
						return []byte("bar"), nil
					case "show -s --format=%B commit3":
						return []byte("foo"), nil
					case "blame --line-porcelain -- foo/c1a.yml":
						return blame(map[string][]blameRange{
							"foo/c1a.yml": {
								{sha: "commit1", lines: []int{2, 12}}, // 1 & 3
								{sha: "commitX", lines: []int{1, 3, 4, 5, 6, 7, 8, 9, 10, 11}},
							},
						}), nil
					case "blame --line-porcelain -- foo/c1b.yml":
						return blame(map[string][]blameRange{
							"foo/c1b.yml": {
								{sha: "commit1", lines: []int{11, 12}}, // 3
								{sha: "commitX", lines: []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}},
							},
						}), nil
					case "blame --line-porcelain -- c2a.yml":
						return blame(map[string][]blameRange{
							"c2a.yml": {
								{sha: "commitX", lines: []int{1, 2, 3, 4, 5, 6, 9, 11, 12}},
								{sha: "commit2", lines: []int{7, 8, 10}}, // 2 & 3
								{sha: "commit3", lines: []int{3}},        // 1
							},
						}), nil
					case "blame --line-porcelain -- c2c.yml":
						return blame(map[string][]blameRange{
							"c2c.yml": {
								{sha: "commit2", lines: []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}}, // 2 & 3
								{sha: "commit3", lines: []int{3}},                                     // 1
							},
						}), nil
					case "blame --line-porcelain -- c3b.yml":
						return blame(map[string][]blameRange{
							"c3b.yml": {
								{sha: "commitX", lines: []int{1, 11, 12}},
								{sha: "commit3", lines: []int{2, 3, 4, 5, 6, 7, 8, 9, 10}}, // 1 & 2 & 3
							},
						}), nil
					case "blame --line-porcelain -- c3c.yml":
						return blame(map[string][]blameRange{
							"c3c.yml": {
								{sha: "commit3", lines: []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}}, // 1 & 2 & 3
							},
						}), nil
					case "blame --line-porcelain -- c3d.yml":
						return blame(map[string][]blameRange{
							"c3d.yml": {
								{sha: "commitX", lines: []int{1, 11, 12}},
								{sha: "commit3", lines: []int{2, 3, 4, 5, 6, 7, 8, 9, 10}}, // 1 & 2 & 3
							},
						}), nil
					default:
						t.Errorf("unknown args: %v", args)
						t.FailNow()
						return nil, nil
					}
				},
				[]*regexp.Regexp{
					regexp.MustCompile("^foo/.*"),
					regexp.MustCompile("^c.*.yml$"),
				},
				"main",
				0,
				[]*regexp.Regexp{regexp.MustCompile(".*")},
			),
			rules: []rule{
				{path: "c3b.yml", name: "first", lines: []int{2, 3}, modified: []int{2, 3}},
				{path: "c3b.yml", name: "second", lines: []int{5, 6, 7, 8}, modified: []int{5, 6, 7, 8}},
				{path: "c3b.yml", name: "third", lines: []int{10, 11, 12, 13}, modified: []int{10}},
				{path: "c3c.yml", name: "first", lines: []int{2, 3}, modified: []int{2, 3}},
				{path: "c3c.yml", name: "second", lines: []int{5, 6, 7, 8}, modified: []int{5, 6, 7, 8}},
				{path: "c3c.yml", name: "third", lines: []int{10, 11, 12, 13}, modified: []int{10, 11, 12}},
				{path: "c3d.yml", name: "first", lines: []int{2, 3}, modified: []int{2, 3}},
				{path: "c3d.yml", name: "second", lines: []int{5, 6, 7, 8}, modified: []int{5, 6, 7, 8}},
				{path: "c3d.yml", name: "third", lines: []int{10, 11, 12, 13}, modified: []int{10}},
				{path: "foo/c1a.yml", name: "first", lines: []int{2, 3}, modified: []int{2}},
				{path: "foo/c1a.yml", name: "third", lines: []int{10, 11, 12, 13}, modified: []int{12}},
				{path: "foo/c1b.yml", name: "third", lines: []int{10, 11, 12, 13}, modified: []int{11, 12}},
				{path: "c2a.yml", name: "first", lines: []int{2, 3}, modified: []int{3}},
				{path: "c2a.yml", name: "second", lines: []int{5, 6, 7, 8}, modified: []int{7, 8}},
				{path: "c2a.yml", name: "third", lines: []int{10, 11, 12, 13}, modified: []int{10}},
				{path: "c2c.yml", name: "first", lines: []int{2, 3}, modified: []int{2, 3, 3}},
				{path: "c2c.yml", name: "second", lines: []int{5, 6, 7, 8}, modified: []int{5, 6, 7, 8}},
				{path: "c2c.yml", name: "third", lines: []int{10, 11, 12, 13}, modified: []int{10, 11, 12}},
			},
		},
		{
			files: map[string]string{
				"foo.yml": testRuleBody,
			},
			finder: discovery.NewGitBranchFinder(
				func(args ...string) ([]byte, error) {
					switch strings.Join(args, " ") {
					case "log --format=%H --no-abbrev-commit --reverse main..HEAD":
						return []byte("commit1\n"), nil
					case "log --reverse --no-merges --pretty=format:%H --name-status commit1^..commit1":
						return []byte("commit1\nM       foo.yml\n"), nil
					case "show -s --format=%B commit1":
						return []byte("foo"), nil
					case "blame --line-porcelain -- foo.yml":
						return blame(map[string][]blameRange{
							"foo.yml": {
								{sha: "commitX", lines: []int{1, 3, 4, 5, 6, 9, 10, 11, 12}},
								{sha: "commit1", lines: []int{2, 7, 8}},
							},
						}), nil
					default:
						t.Errorf("unknown args: %v", args)
						t.FailNow()
						return nil, nil
					}
				},
				nil,
				"main",
				0,
				nil,
			),
			rules: []rule{
				{path: "foo.yml", modified: []int{2, 7, 8}},
			},
		},
		{
			files: map[string]string{
				"foo.yml": testRuleBody,
			},
			finder: discovery.NewGitBranchFinder(
				func(args ...string) ([]byte, error) {
					switch strings.Join(args, " ") {
					case "log --format=%H --no-abbrev-commit --reverse main..HEAD":
						return []byte("commit1\n"), nil
					case "log --reverse --no-merges --pretty=format:%H --name-status commit1^..commit1":
						return []byte("commit1\nM       foo.yml\n"), nil
					case "show -s --format=%B commit1":
						return []byte("foo [skip ci] bar"), nil
					default:
						t.Errorf("unknown args: %v", args)
						t.FailNow()
						return nil, nil
					}
				},
				nil,
				"main",
				0,
				[]*regexp.Regexp{regexp.MustCompile(".*")},
			),
			rules: nil,
		},
		{
			files: map[string]string{
				"foo.yml": testRuleBody,
			},
			finder: discovery.NewGitBranchFinder(
				func(args ...string) ([]byte, error) {
					switch strings.Join(args, " ") {
					case "log --format=%H --no-abbrev-commit --reverse main..HEAD":
						return []byte("commit1\n"), nil
					case "log --reverse --no-merges --pretty=format:%H --name-status commit1^..commit1":
						return []byte("commit1\nM       foo.yml\n"), nil
					case "show -s --format=%B commit1":
						return []byte("foo [no ci] bar"), nil
					default:
						t.Errorf("unknown args: %v", args)
						t.FailNow()
						return nil, nil
					}
				},
				nil,
				"main",
				0,
				[]*regexp.Regexp{regexp.MustCompile(".*")},
			),
			rules: nil,
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			workdir := t.TempDir()
			err := os.Chdir(workdir)
			require.NoError(t, err)

			for p, content := range tc.files {
				if strings.Contains(p, "/") {
					err = os.MkdirAll(path.Dir(p), 0o755)
					require.NoError(t, err)
				}
				err = os.WriteFile(p, []byte(content), 0o644)
				require.NoError(t, err)
			}

			entries, err := tc.finder.Find()
			if tc.err != "" {
				require.EqualError(t, err, tc.err)
			} else {
				require.NoError(t, err)

				rules := []rule{}
				for _, e := range entries {
					t.Logf("Entry: path=%s pathErr=%v lines=%v modified=%v", e.Path, e.PathError, e.Rule.Lines(), e.ModifiedLines)
					var name string
					if e.Rule.AlertingRule != nil {
						name = e.Rule.AlertingRule.Alert.Value.Value
					}
					if e.Rule.RecordingRule != nil {
						name = e.Rule.RecordingRule.Record.Value.Value
					}
					rules = append(rules, rule{
						path:     e.Path,
						name:     name,
						lines:    e.Rule.Lines(),
						modified: e.ModifiedLines,
					})
				}
				require.ElementsMatch(t, tc.rules, rules)
			}
		})
	}
}
