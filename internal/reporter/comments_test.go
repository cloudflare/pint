package reporter

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/neilotoole/slogt"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"
)

var (
	errSummary = errors.New("Summary() error")
	errList    = errors.New("List() error")
	errCreate  = errors.New("Create() error")
	errDelete  = errors.New("Delete() error")
)

type testCommenter struct {
	destinations func(context.Context) ([]any, error)
	summary      func(context.Context, any, Summary, []error) error
	list         func(context.Context, any) ([]ExistingComment, error)
	create       func(context.Context, any, PendingComment) error
	delete       func(context.Context, any, ExistingComment) error
	canCreate    func(int) bool
	isEqual      func(ExistingComment, PendingComment) bool
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

func (tc testCommenter) List(ctx context.Context, dst any) ([]ExistingComment, error) {
	return tc.list(ctx, dst)
}

func (tc testCommenter) Create(ctx context.Context, dst any, comment PendingComment) error {
	return tc.create(ctx, dst, comment)
}

func (tc testCommenter) Delete(ctx context.Context, dst any, comment ExistingComment) error {
	return tc.delete(ctx, dst, comment)
}

func (tc testCommenter) CanDelete(ExistingComment) bool {
	return true
}

func (tc testCommenter) CanCreate(n int) bool {
	return tc.canCreate(n)
}

func (tc testCommenter) IsEqual(_ any, e ExistingComment, p PendingComment) bool {
	return tc.isEqual(e, p)
}

func TestCommenter(t *testing.T) {
	p := parser.NewParser(false, parser.PrometheusSchema, model.UTF8Validation)
	mockFile := p.Parse(strings.NewReader(`
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
		Rule:          mockFile.Groups[0].Rules[0],
		Problem: checks.Problem{
			Reporter: "foo",
			Summary:  "foo error",
			Details:  "foo details",
			Lines:    diags.LineRange{First: 1, Last: 3},
			Severity: checks.Fatal,
			Anchor:   checks.AnchorAfter,
		},
	}
	fooComment := ExistingComment{
		path: "foo.txt",
		line: 2,
		text: `:stop_sign: **Fatal** reported by [pint](https://cloudflare.github.io/pint/) **foo** check.

------

foo error

<details>
<summary>More information</summary>
foo details
</details>

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
		Rule:          mockFile.Groups[0].Rules[0],
		Problem: checks.Problem{
			Reporter: "bar",
			Summary:  "bar warning",
			Details:  "",
			Lines:    diags.LineRange{First: 1, Last: 1},
			Severity: checks.Warning,
			Anchor:   checks.AnchorBefore,
		},
	}
	barComment := ExistingComment{
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
				list: func(_ context.Context, _ any) ([]ExistingComment, error) {
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
				list: func(_ context.Context, _ any) ([]ExistingComment, error) {
					return []ExistingComment{fooComment, barComment}, nil
				},
				create: func(_ context.Context, _ any, p PendingComment) error {
					return fmt.Errorf("shouldn't try to create %s:%d", p.path, p.line)
				},
				delete: func(_ context.Context, _ any, e ExistingComment) error {
					return fmt.Errorf("shouldn't try to delete %s:%d", e.path, e.line)
				},
				isEqual: func(e ExistingComment, p PendingComment) bool {
					return e.path == p.path && e.line == p.line && e.text == p.text
				},
				canCreate: func(_ int) bool {
					return true
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
				list: func(_ context.Context, _ any) ([]ExistingComment, error) {
					return []ExistingComment{fooComment}, nil
				},
				create: func(_ context.Context, _ any, p PendingComment) error {
					if p.path == barComment.path && p.line == barComment.line && p.text == barComment.text {
						return nil
					}
					return fmt.Errorf("shouldn't try to create %s:%d", p.path, p.line)
				},
				delete: func(_ context.Context, _ any, e ExistingComment) error {
					return fmt.Errorf("shouldn't try to delete %s:%d", e.path, e.line)
				},
				isEqual: func(e ExistingComment, p PendingComment) bool {
					return e.path == p.path && e.line == p.line && e.text == p.text
				},
				canCreate: func(_ int) bool {
					return true
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
				list: func(_ context.Context, _ any) ([]ExistingComment, error) {
					return nil, nil
				},
				create: func(_ context.Context, _ any, p PendingComment) error {
					if p.path == barComment.path && p.line == barComment.line && p.text == barComment.text {
						return nil
					}
					return fmt.Errorf("shouldn't try to create %s:%d", p.path, p.line)
				},
				delete: func(_ context.Context, _ any, e ExistingComment) error {
					return fmt.Errorf("shouldn't try to delete %s:%d", e.path, e.line)
				},
				isEqual: func(e ExistingComment, p PendingComment) bool {
					return e.path == p.path && e.line == p.line && e.text == p.text
				},
				canCreate: func(n int) bool {
					return n == 0
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
				list: func(_ context.Context, _ any) ([]ExistingComment, error) {
					return nil, nil
				},
				create: func(_ context.Context, _ any, p PendingComment) error {
					if p.path == barComment.path && p.line == barComment.line && p.text == barComment.text {
						return nil
					}
					return errCreate
				},
				delete: func(_ context.Context, _ any, e ExistingComment) error {
					return fmt.Errorf("shouldn't try to delete %s:%d", e.path, e.line)
				},
				isEqual: func(e ExistingComment, p PendingComment) bool {
					return e.path == p.path && e.line == p.line && e.text == p.text
				},
				canCreate: func(_ int) bool {
					return true
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
				list: func(_ context.Context, _ any) ([]ExistingComment, error) {
					return nil, nil
				},
				create: func(_ context.Context, _ any, p PendingComment) error {
					if p.path == barComment.path && p.line == barComment.line && p.text == barComment.text {
						return nil
					}
					if p.path == fooComment.path && p.line == fooComment.line && p.text == fooComment.text {
						return nil
					}
					return fmt.Errorf("unexpected comment at %s:%d: %s", p.path, p.line, cmp.Diff(fooComment.text, p.text))
				},
				delete: func(_ context.Context, _ any, e ExistingComment) error {
					return fmt.Errorf("shouldn't try to delete %s:%d", e.path, e.line)
				},
				isEqual: func(e ExistingComment, p PendingComment) bool {
					return e.path == p.path && e.line == p.line && e.text == p.text
				},
				canCreate: func(_ int) bool {
					return true
				},
			},
			checkErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			description: "Create() identical details",
			reports: []Report{
				{
					Path: discovery.Path{
						SymlinkTarget: "bar.txt",
						Name:          "foo.txt",
					},
					ModifiedLines: []int{2, 3, 4, 5},
					Rule:          mockFile.Groups[0].Rules[0],
					Problem: checks.Problem{
						Reporter: "foo",
						Summary:  "foo error 1",
						Details:  "foo details",
						Lines:    diags.LineRange{First: 1, Last: 3},
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
					Rule:          mockFile.Groups[0].Rules[0],
					Problem: checks.Problem{
						Reporter: "foo",
						Summary:  "foo error 2",
						Details:  "foo details",
						Lines:    diags.LineRange{First: 1, Last: 3},
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
				list: func(_ context.Context, _ any) ([]ExistingComment, error) {
					return nil, nil
				},
				create: func(_ context.Context, _ any, p PendingComment) error {
					if p.path != "bar.txt" {
						return fmt.Errorf("wrong path: %s", p.path)
					}
					if p.line != 3 {
						return fmt.Errorf("wrong line: %d", p.line)
					}
					expected := `:stop_sign: **Bug** reported by [pint](https://cloudflare.github.io/pint/) **foo** check.

------

foo error 1

<details>
<summary>More information</summary>
foo details
</details>

:leftwards_arrow_with_hook: This problem was detected on a symlinked file ` + "`foo.txt`" + `.

------

foo error 2

<details>
<summary>More information</summary>
foo details
</details>

:leftwards_arrow_with_hook: This problem was detected on a symlinked file ` + "`foo.txt`" + `.

------

:information_source: To see documentation covering this check and instructions on how to resolve it [click here](https://cloudflare.github.io/pint/checks/foo.html).
`
					if p.text != expected {
						return fmt.Errorf("wrong text: %s", cmp.Diff(expected, p.text))
					}
					return nil
				},
				delete: func(_ context.Context, _ any, e ExistingComment) error {
					return fmt.Errorf("shouldn't try to delete %s:%d", e.path, e.line)
				},
				isEqual: func(e ExistingComment, p PendingComment) bool {
					return e.path == p.path && e.line == p.line && e.text == p.text
				},
				canCreate: func(_ int) bool {
					return true
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
					Rule:          mockFile.Groups[0].Rules[0],
					Problem: checks.Problem{
						Reporter: "foo",
						Summary:  "foo error",
						Details:  "foo details",
						Lines:    diags.LineRange{First: 1, Last: 3},
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
				list: func(_ context.Context, _ any) ([]ExistingComment, error) {
					return nil, nil
				},
				create: func(_ context.Context, _ any, p PendingComment) error {
					if p.path != "bar.txt" {
						return fmt.Errorf("wrong path: %s", p.path)
					}
					if p.line != 3 {
						return fmt.Errorf("wrong line: %d", p.line)
					}
					expected := `:stop_sign: **Bug** reported by [pint](https://cloudflare.github.io/pint/) **foo** check.

------

foo error

<details>
<summary>More information</summary>
foo details
</details>

:leftwards_arrow_with_hook: This problem was detected on a symlinked file ` + "`foo.txt`" + `.

------

:information_source: To see documentation covering this check and instructions on how to resolve it [click here](https://cloudflare.github.io/pint/checks/foo.html).
`
					if p.text != expected {
						return fmt.Errorf("wrong text: %s", cmp.Diff(expected, p.text))
					}
					return nil
				},
				delete: func(_ context.Context, _ any, e ExistingComment) error {
					return fmt.Errorf("shouldn't try to delete %s:%d", e.path, e.line)
				},
				isEqual: func(e ExistingComment, p PendingComment) bool {
					return e.path == p.path && e.line == p.line && e.text == p.text
				},
				canCreate: func(_ int) bool {
					return true
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
					Rule:          mockFile.Groups[0].Rules[0],
					Problem: checks.Problem{
						Reporter: "foo",
						Summary:  "foo error 1",
						Details:  "foo details",
						Lines:    diags.LineRange{First: 1, Last: 3},
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
					Rule:          mockFile.Groups[0].Rules[0],
					Problem: checks.Problem{
						Reporter: "foo",
						Summary:  "foo error 2",
						Details:  "foo details",
						Lines:    diags.LineRange{First: 2, Last: 2},
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
				list: func(_ context.Context, _ any) ([]ExistingComment, error) {
					return nil, nil
				},
				create: func(_ context.Context, _ any, p PendingComment) error {
					if p.path != "foo.txt" {
						return fmt.Errorf("wrong path: %s", p.path)
					}
					if p.line != 2 && p.line != 3 {
						return fmt.Errorf("wrong line: %d", p.line)
					}
					expected := `:stop_sign: **Bug** reported by [pint](https://cloudflare.github.io/pint/) **foo** check.

------

foo error 1

<details>
<summary>More information</summary>
foo details
</details>

------

:information_source: To see documentation covering this check and instructions on how to resolve it [click here](https://cloudflare.github.io/pint/checks/foo.html).
`
					if p.line == 3 && p.text != expected {
						return fmt.Errorf("wrong text on first report: %s", cmp.Diff(expected, p.text))
					}
					expected2 := `:stop_sign: **Bug** reported by [pint](https://cloudflare.github.io/pint/) **foo** check.

------

foo error 2

<details>
<summary>More information</summary>
foo details
</details>

------

:information_source: To see documentation covering this check and instructions on how to resolve it [click here](https://cloudflare.github.io/pint/checks/foo.html).
`
					if p.line == 2 && p.text != expected2 {
						return fmt.Errorf("wrong text on second report: %s", cmp.Diff(expected2, p.text))
					}
					return nil
				},
				delete: func(_ context.Context, _ any, e ExistingComment) error {
					return fmt.Errorf("shouldn't try to delete %s:%d", e.path, e.line)
				},
				isEqual: func(e ExistingComment, p PendingComment) bool {
					return e.path == p.path && e.line == p.line && e.text == p.text
				},
				canCreate: func(_ int) bool {
					return true
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
				list: func(_ context.Context, _ any) ([]ExistingComment, error) {
					return []ExistingComment{fooComment}, nil
				},
				create: func(_ context.Context, _ any, _ PendingComment) error {
					return nil
				},
				delete: func(_ context.Context, _ any, _ ExistingComment) error {
					return errDelete
				},
				isEqual: func(e ExistingComment, p PendingComment) bool {
					return e.path == p.path && e.line == p.line && e.text == p.text
				},
				canCreate: func(_ int) bool {
					return true
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
				list: func(_ context.Context, _ any) ([]ExistingComment, error) {
					return []ExistingComment{fooComment}, nil
				},
				create: func(_ context.Context, _ any, _ PendingComment) error {
					return nil
				},
				delete: func(_ context.Context, _ any, _ ExistingComment) error {
					return nil
				},
				isEqual: func(e ExistingComment, p PendingComment) bool {
					return e.path == p.path && e.line == p.line && e.text == p.text
				},
				canCreate: func(_ int) bool {
					return true
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
				list: func(_ context.Context, _ any) ([]ExistingComment, error) {
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
			tc.checkErr(t, Submit(t.Context(), summary, tc.commenter))
		})
	}
}

func TestCommentsCommonPaths(t *testing.T) {
	type errorCheck func(err error) error

	type testCaseT struct {
		httpHandler  http.Handler
		errorHandler errorCheck

		description string
		branch      string
		token       string

		reports     []Report
		timeout     time.Duration
		project     int
		maxComments int
	}

	p := parser.NewParser(false, parser.PrometheusSchema, model.UTF8Validation)
	mockFile := p.Parse(strings.NewReader(`
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
		Rule:          mockFile.Groups[0].Rules[0],
		Problem: checks.Problem{
			Reporter: "foo",
			Summary:  "foo error",
			Details:  "foo details",
			Lines:    diags.LineRange{First: 1, Last: 3},
			Severity: checks.Fatal,
			Anchor:   checks.AnchorAfter,
		},
	}

	testCases := []testCaseT{
		{
			description: "returns an error on non-200 HTTP response",
			branch:      "fakeBranch",
			token:       "fakeToken",
			timeout:     time.Second,
			project:     123,
			maxComments: 50,
			reports:     []Report{fooReport},
			httpHandler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte("Bad Request"))
			}),
			errorHandler: func(err error) error {
				if err != nil {
					return nil
				}
				return fmt.Errorf("wrong error: %w", err)
			},
		},
		{
			description: "returns an error on HTTP response timeout",
			branch:      "fakeBranch",
			token:       "fakeToken",
			timeout:     time.Second,
			project:     123,
			maxComments: 50,
			reports:     []Report{fooReport},
			httpHandler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				time.Sleep(time.Second * 2)
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte("Bad Request"))
			}),
			errorHandler: func(err error) error {
				if err != nil && strings.HasSuffix(err.Error(), "context deadline exceeded") {
					return nil
				}
				return fmt.Errorf("wrong error: %w", err)
			},
		},
		{
			description: "returns an error on non-json body",
			branch:      "fakeBranch",
			token:       "fakeToken",
			timeout:     time.Second,
			project:     123,
			maxComments: 50,
			reports:     []Report{fooReport},
			httpHandler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("OK"))
			}),
			errorHandler: func(err error) error {
				if err != nil {
					return nil
				}
				return fmt.Errorf("wrong error: %w", err)
			},
		},
		{
			description: "returns an error on empty JSON body",
			branch:      "fakeBranch",
			token:       "fakeToken",
			timeout:     time.Second,
			project:     123,
			maxComments: 50,
			reports:     []Report{fooReport},
			httpHandler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("{}"))
			}),
			errorHandler: func(err error) error {
				if err != nil {
					return nil
				}
				return fmt.Errorf("wrong error: %w", err)
			},
		},
	}

	for _, tc := range testCases {
		for _, c := range []func(string) Commenter{
			func(uri string) Commenter {
				r, err := NewGitLabReporter(
					"v0.0.0",
					tc.branch,
					uri,
					tc.timeout,
					tc.token,
					tc.project,
					tc.maxComments,
				)
				require.NoError(t, err, "can't create gitlab reporter")
				return r
			},
			func(uri string) Commenter {
				r, err := NewGithubReporter(
					"v0.0.0",
					uri,
					uri,
					tc.timeout,
					tc.token,
					"owner",
					"repo",
					123,
					tc.maxComments,
					"fake-commit-id",
				)
				require.NoError(t, err, "can't create gitlab reporter")
				return r
			},
		} {
			t.Run(tc.description, func(t *testing.T) {
				slog.SetDefault(slogt.New(t))

				srv := httptest.NewServer(tc.httpHandler)
				defer srv.Close()

				summary := NewSummary(tc.reports)
				err := Submit(t.Context(), summary, c(srv.URL))
				require.NoError(t, tc.errorHandler(err))
			})
		}
	}
}
