package reporter

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"

	"github.com/neilotoole/slogt"
	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"
)

var (
	errSummary   = errors.New("Summary() error")
	errList      = errors.New("List() error")
	errCanCreate = errors.New("CanCreate() error")
	errCreate    = errors.New("Create() error")
	errDelete    = errors.New("Delete() error")
)

type testCommenter struct {
	destinations func(context.Context) ([]any, error)
	summary      func(context.Context, any, Summary, []error) error
	list         func(context.Context, any) ([]ExistingCommentV2, error)
	create       func(context.Context, any, PendingCommentV2) error
	delete       func(context.Context, any, ExistingCommentV2) error
	canCreate    func(int) (bool, error)
	isEqual      func(ExistingCommentV2, PendingCommentV2) bool
}

func (tc testCommenter) Describe() string {
	return "testCommenter"
}

func (tc testCommenter) Destinations(ctx context.Context) ([]any, error) {
	return tc.destinations(ctx)
}

func (tc testCommenter) Summary(ctx context.Context, dst any, s Summary, errs []error) error {
	return tc.summary(ctx, dst, s, errs)
}

func (tc testCommenter) List(ctx context.Context, dst any) ([]ExistingCommentV2, error) {
	return tc.list(ctx, dst)
}

func (tc testCommenter) Create(ctx context.Context, dst any, comment PendingCommentV2) error {
	return tc.create(ctx, dst, comment)
}

func (tc testCommenter) Delete(ctx context.Context, dst any, comment ExistingCommentV2) error {
	return tc.delete(ctx, dst, comment)
}

func (tc testCommenter) CanCreate(n int) (bool, error) {
	return tc.canCreate(n)
}

func (tc testCommenter) IsEqual(e ExistingCommentV2, p PendingCommentV2) bool {
	return tc.isEqual(e, p)
}

func TestCommenter(t *testing.T) {
	p := parser.NewParser(false)
	mockRules, _ := p.Parse([]byte(`
- record: target is down
  expr: up == 0
- record: sum errors
  expr: sum(errors) by (job)
`))

	fooReport := Report{
		Path: discovery.Path{
			SymlinkTarget: "foo.txt",
			Name:          "foo.txt",
		},
		ModifiedLines: []int{2},
		Rule:          mockRules[0],
		Problem: checks.Problem{
			Reporter: "foo",
			Text:     "foo error",
			Details:  "foo details",
			Lines:    parser.LineRange{First: 1, Last: 3},
			Severity: checks.Fatal,
			Anchor:   checks.AnchorAfter,
		},
	}
	fooComment := ExistingCommentV2{
		path: "foo.txt",
		line: 2,
		text: `:stop_sign: **Fatal** reported by [pint](https://cloudflare.github.io/pint/) **foo** check.

------

foo error

foo details

------

:information_source: To see documentation covering this check and instructions on how to resolve it [click here](https://cloudflare.github.io/pint/checks/foo.html).
`,
		meta: nil,
	}

	barReport := Report{
		Path: discovery.Path{
			SymlinkTarget: "bar.txt",
			Name:          "bar.txt",
		},
		ModifiedLines: []int{1},
		Rule:          mockRules[0],
		Problem: checks.Problem{
			Reporter: "bar",
			Text:     "bar warning",
			Details:  "",
			Lines:    parser.LineRange{First: 1, Last: 1},
			Severity: checks.Warning,
			Anchor:   checks.AnchorBefore,
		},
	}
	barComment := ExistingCommentV2{
		path: "bar.txt",
		line: 1,
		text: `:warning: **Warning** reported by [pint](https://cloudflare.github.io/pint/) **bar** check.

------

bar warning

------

:information_source: To see documentation covering this check and instructions on how to resolve it [click here](https://cloudflare.github.io/pint/checks/bar.html).
`,
		meta: nil,
	}

	type testCaseT struct {
		commenter   Commenter
		checkErr    func(t *testing.T, err error)
		description string
		reports     []Report
	}

	testCases := []testCaseT{
		{
			description: "no-op when there are no destinations",
			reports:     []Report{},
			commenter: testCommenter{
				destinations: func(_ context.Context) ([]any, error) {
					return []any{}, nil
				},
			},
			checkErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			description: "stops if List() fails",
			reports:     []Report{},
			commenter: testCommenter{
				destinations: func(_ context.Context) ([]any, error) {
					return []any{1}, nil
				},
				summary: func(_ context.Context, _ any, _ Summary, errs []error) error {
					if len(errs) != 0 {
						return fmt.Errorf("Expected empty errs, got %v", errs)
					}
					return nil
				},
				list: func(_ context.Context, _ any) ([]ExistingCommentV2, error) {
					return nil, errList
				},
			},
			checkErr: func(t *testing.T, err error) {
				require.ErrorIs(t, err, errList)
			},
		},
		{
			description: "no-op when all comments already exist",
			reports:     []Report{fooReport, barReport},
			commenter: testCommenter{
				destinations: func(_ context.Context) ([]any, error) {
					return []any{1}, nil
				},
				summary: func(_ context.Context, _ any, _ Summary, errs []error) error {
					if len(errs) != 0 {
						return fmt.Errorf("Expected empty errs, got %v", errs)
					}
					return nil
				},
				list: func(_ context.Context, _ any) ([]ExistingCommentV2, error) {
					return []ExistingCommentV2{fooComment, barComment}, nil
				},
				create: func(_ context.Context, _ any, p PendingCommentV2) error {
					return fmt.Errorf("shouldn't try to create %s:%d", p.path, p.line)
				},
				delete: func(_ context.Context, _ any, e ExistingCommentV2) error {
					return fmt.Errorf("shouldn't try to delete %s:%d", e.path, e.line)
				},
				isEqual: func(e ExistingCommentV2, p PendingCommentV2) bool {
					return e.path == p.path && e.line == p.line && e.text == p.text
				},
				canCreate: func(_ int) (bool, error) {
					return true, nil
				},
			},
			checkErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			description: "creates missing comments",
			reports:     []Report{fooReport, barReport},
			commenter: testCommenter{
				destinations: func(_ context.Context) ([]any, error) {
					return []any{1}, nil
				},
				summary: func(_ context.Context, _ any, _ Summary, errs []error) error {
					if len(errs) != 0 {
						return fmt.Errorf("Expected empty errs, got %v", errs)
					}
					return nil
				},
				list: func(_ context.Context, _ any) ([]ExistingCommentV2, error) {
					return []ExistingCommentV2{fooComment}, nil
				},
				create: func(_ context.Context, _ any, p PendingCommentV2) error {
					if p.path == barComment.path && p.line == barComment.line && p.text == barComment.text {
						return nil
					}
					return fmt.Errorf("shouldn't try to create %s:%d", p.path, p.line)
				},
				delete: func(_ context.Context, _ any, e ExistingCommentV2) error {
					return fmt.Errorf("shouldn't try to delete %s:%d", e.path, e.line)
				},
				isEqual: func(e ExistingCommentV2, p PendingCommentV2) bool {
					return e.path == p.path && e.line == p.line && e.text == p.text
				},
				canCreate: func(_ int) (bool, error) {
					return true, nil
				},
			},
			checkErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			description: "skips creating comments when CanCreate() is false",
			reports:     []Report{barReport, fooReport},
			commenter: testCommenter{
				destinations: func(_ context.Context) ([]any, error) {
					return []any{1}, nil
				},
				summary: func(_ context.Context, _ any, _ Summary, errs []error) error {
					if len(errs) != 0 {
						return fmt.Errorf("Expected empty errs, got %v", errs)
					}
					return nil
				},
				list: func(_ context.Context, _ any) ([]ExistingCommentV2, error) {
					return nil, nil
				},
				create: func(_ context.Context, _ any, p PendingCommentV2) error {
					if p.path == barComment.path && p.line == barComment.line && p.text == barComment.text {
						return nil
					}
					return fmt.Errorf("shouldn't try to create %s:%d", p.path, p.line)
				},
				delete: func(_ context.Context, _ any, e ExistingCommentV2) error {
					return fmt.Errorf("shouldn't try to delete %s:%d", e.path, e.line)
				},
				isEqual: func(e ExistingCommentV2, p PendingCommentV2) bool {
					return e.path == p.path && e.line == p.line && e.text == p.text
				},
				canCreate: func(n int) (bool, error) {
					return n == 0, nil
				},
			},
			checkErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			description: "CanCreate() fails",
			reports:     []Report{barReport, fooReport},
			commenter: testCommenter{
				destinations: func(_ context.Context) ([]any, error) {
					return []any{1}, nil
				},
				summary: func(_ context.Context, _ any, _ Summary, errs []error) error {
					if len(errs) != 2 {
						return fmt.Errorf("Expected errCanCreate in errs, got %v", errs)
					}
					if !errors.Is(errs[0], errCanCreate) {
						return fmt.Errorf("Expected errCanCreate in errs, got %w", errs[0])
					}
					if !errors.Is(errs[1], errCanCreate) {
						return fmt.Errorf("Expected errCanCreate in errs, got %w", errs[1])
					}
					return nil
				},
				list: func(_ context.Context, _ any) ([]ExistingCommentV2, error) {
					return nil, nil
				},
				create: func(_ context.Context, _ any, p PendingCommentV2) error {
					if p.path == barComment.path && p.line == barComment.line && p.text == barComment.text {
						return nil
					}
					return fmt.Errorf("shouldn't try to create %s:%d", p.path, p.line)
				},
				delete: func(_ context.Context, _ any, e ExistingCommentV2) error {
					return fmt.Errorf("shouldn't try to delete %s:%d", e.path, e.line)
				},
				isEqual: func(e ExistingCommentV2, p PendingCommentV2) bool {
					return e.path == p.path && e.line == p.line && e.text == p.text
				},
				canCreate: func(_ int) (bool, error) {
					return false, errCanCreate
				},
			},
			checkErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			description: "Create() fails",
			reports:     []Report{barReport, fooReport},
			commenter: testCommenter{
				destinations: func(_ context.Context) ([]any, error) {
					return []any{1}, nil
				},
				summary: func(_ context.Context, _ any, _ Summary, errs []error) error {
					if len(errs) != 0 {
						return fmt.Errorf("Expected empty errs, got %v", errs)
					}
					return nil
				},
				list: func(_ context.Context, _ any) ([]ExistingCommentV2, error) {
					return nil, nil
				},
				create: func(_ context.Context, _ any, p PendingCommentV2) error {
					if p.path == barComment.path && p.line == barComment.line && p.text == barComment.text {
						return nil
					}
					return errCreate
				},
				delete: func(_ context.Context, _ any, e ExistingCommentV2) error {
					return fmt.Errorf("shouldn't try to delete %s:%d", e.path, e.line)
				},
				isEqual: func(e ExistingCommentV2, p PendingCommentV2) bool {
					return e.path == p.path && e.line == p.line && e.text == p.text
				},
				canCreate: func(_ int) (bool, error) {
					return true, nil
				},
			},
			checkErr: func(t *testing.T, err error) {
				require.ErrorIs(t, err, errCreate)
			},
		},
		{
			description: "Create() works",
			reports:     []Report{barReport, fooReport},
			commenter: testCommenter{
				destinations: func(_ context.Context) ([]any, error) {
					return []any{1}, nil
				},
				summary: func(_ context.Context, _ any, _ Summary, errs []error) error {
					if len(errs) != 0 {
						return fmt.Errorf("Expected empty errs, got %v", errs)
					}
					return nil
				},
				list: func(_ context.Context, _ any) ([]ExistingCommentV2, error) {
					return nil, nil
				},
				create: func(_ context.Context, _ any, p PendingCommentV2) error {
					if p.path == barComment.path && p.line == barComment.line && p.text == barComment.text {
						return nil
					}
					if p.path == fooComment.path && p.line == fooComment.line && p.text == fooComment.text {
						return nil
					}
					return fmt.Errorf("unexpected comment at %s:%d: %s", p.path, p.line, p.text)
				},
				delete: func(_ context.Context, _ any, e ExistingCommentV2) error {
					return fmt.Errorf("shouldn't try to delete %s:%d", e.path, e.line)
				},
				isEqual: func(e ExistingCommentV2, p PendingCommentV2) bool {
					return e.path == p.path && e.line == p.line && e.text == p.text
				},
				canCreate: func(_ int) (bool, error) {
					return true, nil
				},
			},
			checkErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			description: "Create() merges details",
			reports: []Report{
				{
					Path: discovery.Path{
						SymlinkTarget: "bar.txt",
						Name:          "foo.txt",
					},
					ModifiedLines: []int{2, 3, 4, 5},
					Rule:          mockRules[0],
					Problem: checks.Problem{
						Reporter: "foo",
						Text:     "foo error 1",
						Details:  "foo details",
						Lines:    parser.LineRange{First: 1, Last: 3},
						Severity: checks.Bug,
						Anchor:   checks.AnchorAfter,
					},
				},
				{
					Path: discovery.Path{
						SymlinkTarget: "bar.txt",
						Name:          "foo.txt",
					},
					ModifiedLines: []int{2, 3, 4, 5},
					Rule:          mockRules[0],
					Problem: checks.Problem{
						Reporter: "foo",
						Text:     "foo error 2",
						Details:  "foo details",
						Lines:    parser.LineRange{First: 1, Last: 3},
						Severity: checks.Bug,
						Anchor:   checks.AnchorAfter,
					},
				},
			},
			commenter: testCommenter{
				destinations: func(_ context.Context) ([]any, error) {
					return []any{1}, nil
				},
				summary: func(_ context.Context, _ any, _ Summary, errs []error) error {
					if len(errs) != 0 {
						return fmt.Errorf("Expected empty errs, got %v", errs)
					}
					return nil
				},
				list: func(_ context.Context, _ any) ([]ExistingCommentV2, error) {
					return nil, nil
				},
				create: func(_ context.Context, _ any, p PendingCommentV2) error {
					if p.path != "bar.txt" {
						return fmt.Errorf("wrong path: %s", p.path)
					}
					if p.line != 3 {
						return fmt.Errorf("wrong line: %d", p.line)
					}
					if p.text != `:stop_sign: **Bug** reported by [pint](https://cloudflare.github.io/pint/) **foo** check.

------

foo error 1

:leftwards_arrow_with_hook: This problem was detected on a symlinked file `+"`foo.txt`"+`.

------

foo error 2

:leftwards_arrow_with_hook: This problem was detected on a symlinked file `+"`foo.txt`"+`.

------

foo details

------

:information_source: To see documentation covering this check and instructions on how to resolve it [click here](https://cloudflare.github.io/pint/checks/foo.html).
` {
						return fmt.Errorf("wrong text: %s", p.text)
					}
					return nil
				},
				delete: func(_ context.Context, _ any, e ExistingCommentV2) error {
					return fmt.Errorf("shouldn't try to delete %s:%d", e.path, e.line)
				},
				isEqual: func(e ExistingCommentV2, p PendingCommentV2) bool {
					return e.path == p.path && e.line == p.line && e.text == p.text
				},
				canCreate: func(_ int) (bool, error) {
					return true, nil
				},
			},
			checkErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			description: "Create() reports symlink",
			reports: []Report{
				{
					Path: discovery.Path{
						SymlinkTarget: "bar.txt",
						Name:          "foo.txt",
					},
					ModifiedLines: []int{2, 3, 4, 5},
					Rule:          mockRules[0],
					Problem: checks.Problem{
						Reporter: "foo",
						Text:     "foo error",
						Details:  "foo details",
						Lines:    parser.LineRange{First: 1, Last: 3},
						Severity: checks.Bug,
						Anchor:   checks.AnchorAfter,
					},
				},
			},
			commenter: testCommenter{
				destinations: func(_ context.Context) ([]any, error) {
					return []any{1}, nil
				},
				summary: func(_ context.Context, _ any, _ Summary, errs []error) error {
					if len(errs) != 0 {
						return fmt.Errorf("Expected empty errs, got %v", errs)
					}
					return nil
				},
				list: func(_ context.Context, _ any) ([]ExistingCommentV2, error) {
					return nil, nil
				},
				create: func(_ context.Context, _ any, p PendingCommentV2) error {
					if p.path != "bar.txt" {
						return fmt.Errorf("wrong path: %s", p.path)
					}
					if p.line != 3 {
						return fmt.Errorf("wrong line: %d", p.line)
					}
					if p.text != `:stop_sign: **Bug** reported by [pint](https://cloudflare.github.io/pint/) **foo** check.

------

foo error

foo details

:leftwards_arrow_with_hook: This problem was detected on a symlinked file `+"`foo.txt`"+`.

------

:information_source: To see documentation covering this check and instructions on how to resolve it [click here](https://cloudflare.github.io/pint/checks/foo.html).
` {
						return fmt.Errorf("wrong text: %s", p.text)
					}
					return nil
				},
				delete: func(_ context.Context, _ any, e ExistingCommentV2) error {
					return fmt.Errorf("shouldn't try to delete %s:%d", e.path, e.line)
				},
				isEqual: func(e ExistingCommentV2, p PendingCommentV2) bool {
					return e.path == p.path && e.line == p.line && e.text == p.text
				},
				canCreate: func(_ int) (bool, error) {
					return true, nil
				},
			},
			checkErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			description: "Create() doesn't merge details with different line range",
			reports: []Report{
				{
					Path: discovery.Path{
						SymlinkTarget: "foo.txt",
						Name:          "foo.txt",
					},
					ModifiedLines: []int{2, 3, 4, 5},
					Rule:          mockRules[0],
					Problem: checks.Problem{
						Reporter: "foo",
						Text:     "foo error 1",
						Details:  "foo details",
						Lines:    parser.LineRange{First: 1, Last: 3},
						Severity: checks.Bug,
						Anchor:   checks.AnchorAfter,
					},
				},
				{
					Path: discovery.Path{
						SymlinkTarget: "foo.txt",
						Name:          "foo.txt",
					},
					ModifiedLines: []int{2, 3, 4, 5},
					Rule:          mockRules[0],
					Problem: checks.Problem{
						Reporter: "foo",
						Text:     "foo error 2",
						Details:  "foo details",
						Lines:    parser.LineRange{First: 2, Last: 2},
						Severity: checks.Bug,
						Anchor:   checks.AnchorAfter,
					},
				},
			},
			commenter: testCommenter{
				destinations: func(_ context.Context) ([]any, error) {
					return []any{1}, nil
				},
				summary: func(_ context.Context, _ any, _ Summary, errs []error) error {
					if len(errs) != 0 {
						return fmt.Errorf("Expected empty errs, got %v", errs)
					}
					return nil
				},
				list: func(_ context.Context, _ any) ([]ExistingCommentV2, error) {
					return nil, nil
				},
				create: func(_ context.Context, _ any, p PendingCommentV2) error {
					if p.path != "foo.txt" {
						return fmt.Errorf("wrong path: %s", p.path)
					}
					if p.line != 2 && p.line != 3 {
						return fmt.Errorf("wrong line: %d", p.line)
					}
					if p.line == 3 && p.text != `:stop_sign: **Bug** reported by [pint](https://cloudflare.github.io/pint/) **foo** check.

------

foo error 1

foo details

------

:information_source: To see documentation covering this check and instructions on how to resolve it [click here](https://cloudflare.github.io/pint/checks/foo.html).
` {
						return fmt.Errorf("wrong text on first report: %s", p.text)
					}
					if p.line == 2 && p.text != `:stop_sign: **Bug** reported by [pint](https://cloudflare.github.io/pint/) **foo** check.

------

foo error 2

foo details

------

:information_source: To see documentation covering this check and instructions on how to resolve it [click here](https://cloudflare.github.io/pint/checks/foo.html).
` {
						return fmt.Errorf("wrong text on second report: %s", p.text)
					}
					return nil
				},
				delete: func(_ context.Context, _ any, e ExistingCommentV2) error {
					return fmt.Errorf("shouldn't try to delete %s:%d", e.path, e.line)
				},
				isEqual: func(e ExistingCommentV2, p PendingCommentV2) bool {
					return e.path == p.path && e.line == p.line && e.text == p.text
				},
				canCreate: func(_ int) (bool, error) {
					return true, nil
				},
			},
			checkErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			description: "Delete() fails",
			reports:     []Report{barReport},
			commenter: testCommenter{
				destinations: func(_ context.Context) ([]any, error) {
					return []any{1}, nil
				},
				summary: func(_ context.Context, _ any, _ Summary, errs []error) error {
					if len(errs) != 1 {
						return fmt.Errorf("Expected errDelete in errs, got %v", errs)
					}
					if !errors.Is(errs[0], errDelete) {
						return fmt.Errorf("Expected errDelete in errs, got %w", errs[0])
					}
					return nil
				},
				list: func(_ context.Context, _ any) ([]ExistingCommentV2, error) {
					return []ExistingCommentV2{fooComment}, nil
				},
				create: func(_ context.Context, _ any, _ PendingCommentV2) error {
					return nil
				},
				delete: func(_ context.Context, _ any, _ ExistingCommentV2) error {
					return errDelete
				},
				isEqual: func(e ExistingCommentV2, p PendingCommentV2) bool {
					return e.path == p.path && e.line == p.line && e.text == p.text
				},
				canCreate: func(_ int) (bool, error) {
					return true, nil
				},
			},
			checkErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			description: "Delete() works",
			reports:     []Report{barReport},
			commenter: testCommenter{
				destinations: func(_ context.Context) ([]any, error) {
					return []any{1}, nil
				},
				summary: func(_ context.Context, _ any, _ Summary, errs []error) error {
					if len(errs) != 0 {
						return fmt.Errorf("Expected empty errs, got %v", errs)
					}
					return nil
				},
				list: func(_ context.Context, _ any) ([]ExistingCommentV2, error) {
					return []ExistingCommentV2{fooComment}, nil
				},
				create: func(_ context.Context, _ any, _ PendingCommentV2) error {
					return nil
				},
				delete: func(_ context.Context, _ any, _ ExistingCommentV2) error {
					return nil
				},
				isEqual: func(e ExistingCommentV2, p PendingCommentV2) bool {
					return e.path == p.path && e.line == p.line && e.text == p.text
				},
				canCreate: func(_ int) (bool, error) {
					return true, nil
				},
			},
			checkErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			description: "Summary() fails",
			reports:     []Report{},
			commenter: testCommenter{
				destinations: func(_ context.Context) ([]any, error) {
					return []any{1}, nil
				},
				summary: func(_ context.Context, _ any, _ Summary, _ []error) error {
					return errSummary
				},
				list: func(_ context.Context, _ any) ([]ExistingCommentV2, error) {
					return nil, nil
				},
			},
			checkErr: func(t *testing.T, err error) {
				require.ErrorIs(t, err, errSummary)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			slog.SetDefault(slogt.New(t))

			summary := NewSummary(tc.reports)
			tc.checkErr(t, Submit(context.Background(), summary, tc.commenter))
		})
	}
}
