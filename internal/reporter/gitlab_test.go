package reporter_test

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/neilotoole/slogt"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	gitlab "gitlab.com/gitlab-org/api/client-go"
	"go.nhat.io/httpmock"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/promapi"
	"github.com/cloudflare/pint/internal/reporter"
)

func TestGitLabReporterBadBaseURI(t *testing.T) {
	slog.SetDefault(slogt.New(t))
	_, err := reporter.NewGitLabReporter(
		"v0.0.0",
		"branch",
		"%gh&%ij",
		time.Minute,
		"token",
		123,
		0,
	)
	require.Error(t, err)
}

func TestGitLabReporter(t *testing.T) {
	type errorCheck func(err error) error

	p := parser.NewParser(false, parser.PrometheusSchema, model.UTF8Validation)
	mockRules := `
- record: target is down
  expr: up == 0
- record: sum errors
  expr: sum(errors) by (job)
`
	tmpDir := t.TempDir()
	mockPath := filepath.Join(tmpDir, "foo.txt")
	require.NoError(t, os.WriteFile(mockPath, []byte(mockRules), 0o644))
	mockFile := p.Parse(strings.NewReader(mockRules))

	fooReport := reporter.Report{
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
	fooDiff := `@@ -1,4 +1,6 @@\n- record: target is down\n-  expr: up == 0\n+  expr: up == 1\n+  labels:\n+    foo: bar\n- record: sum errors\nexpr: sum(errors) by (job)\n`

	summaryWithDetails := reporter.NewSummary([]reporter.Report{})
	summaryWithDetails.MarkCheckDisabled("prom1", promapi.APIPathConfig, []string{"check1", "check3", "check2"})
	summaryWithDetails.MarkCheckDisabled("prom2", promapi.APIPathMetadata, []string{"check1"})

	summaryABC := reporter.NewSummary([]reporter.Report{
		{
			Path: discovery.Path{
				SymlinkTarget: "foo.txt",
				Name:          "foo.txt",
			},
			ModifiedLines: []int{1},
			Rule:          mockFile.Groups[0].Rules[0],
			Problem: checks.Problem{
				Reporter: "a",
				Summary:  "foo error1",
				Details:  "foo details",
				Lines:    diags.LineRange{First: 1, Last: 3},
				Severity: checks.Fatal,
				Anchor:   checks.AnchorAfter,
			},
		},
		{
			Path: discovery.Path{
				SymlinkTarget: "foo.txt",
				Name:          "foo.txt",
			},
			ModifiedLines: []int{2},
			Rule:          mockFile.Groups[0].Rules[0],
			Problem: checks.Problem{
				Reporter: "b",
				Summary:  "foo error2",
				Details:  "foo details",
				Lines:    diags.LineRange{First: 1, Last: 3},
				Severity: checks.Fatal,
				Anchor:   checks.AnchorAfter,
			},
		},
		{
			Path: discovery.Path{
				SymlinkTarget: "foo.txt",
				Name:          "foo.txt",
			},
			ModifiedLines: []int{3},
			Rule:          mockFile.Groups[0].Rules[0],
			Problem: checks.Problem{
				Reporter: "c",
				Summary:  "foo error3",
				Details:  "foo details",
				Lines:    diags.LineRange{First: 1, Last: 3},
				Severity: checks.Fatal,
				Anchor:   checks.AnchorAfter,
			},
		},
	})

	const (
		apiUser              = "/api/v4/user"
		apiOpenMergeRequests = "/api/v4/projects/123/merge_requests?page=1&source_branch=fakeBranch&state=opened"
	)
	apiDiffs := func(mrID int) string {
		return fmt.Sprintf("/api/v4/projects/123/merge_requests/%d/diffs?page=1", mrID)
	}
	apiVersions := func(mrID int) string {
		return fmt.Sprintf("/api/v4/projects/123/merge_requests/%d/versions?page=1", mrID)
	}
	apiDiscussions := func(mrID int, withPage bool) string {
		path := fmt.Sprintf("/api/v4/projects/123/merge_requests/%d/discussions", mrID)
		if withPage {
			path += "?page=1"
		}
		return path
	}
	discBody := func(reporter, summary, details string) *string {
		return gitlab.Ptr(fmt.Sprintf(`:stop_sign: **Fatal** reported by [pint](https://cloudflare.github.io/pint/) **%s** check.

------

%s

<details>
<summary>More information</summary>
%s
</details>

------

:information_source: To see documentation covering this check and instructions on how to resolve it [click here](https://cloudflare.github.io/pint/checks/%s.html).
`, reporter, summary, details, reporter))
	}
	discBodyWithDiag := func(reporter, summary, details, yml, diag string) *string {
		return gitlab.Ptr(fmt.Sprintf(
			`:stop_sign: **Fatal** reported by [pint](https://cloudflare.github.io/pint/) **%s** check.

<details>
<summary>%s</summary>

%s

%s

%s

</details>

------

:information_source: To see documentation covering this check and instructions on how to resolve it [click here](https://cloudflare.github.io/pint/checks/a.html).
`, reporter, summary, yml, diag, details))
	}
	discPosition := func(path string, line int) *gitlab.PositionOptions {
		return gitlab.Ptr(gitlab.PositionOptions{
			BaseSHA:      gitlab.Ptr("base"),
			StartSHA:     gitlab.Ptr("start"),
			HeadSHA:      gitlab.Ptr("head"),
			OldPath:      gitlab.Ptr(path),
			NewPath:      gitlab.Ptr(path),
			PositionType: gitlab.Ptr("text"),
			NewLine:      gitlab.Ptr(line),
			OldLine:      gitlab.Ptr(line),
		})
	}
	notePos := func(oldPath, newPath string, newLine, oldLine int) *gitlab.NotePosition {
		return gitlab.Ptr(gitlab.NotePosition{
			BaseSHA:      "base",
			StartSHA:     "start",
			HeadSHA:      "head",
			OldPath:      oldPath,
			NewPath:      newPath,
			PositionType: "text",
			NewLine:      newLine,
			OldLine:      oldLine,
		})
	}
	discNote := func(id, authorID int, body string, pos *gitlab.NotePosition) *gitlab.Note {
		return gitlab.Ptr(gitlab.Note{
			ID:       id,
			Author:   gitlab.NoteAuthor{ID: authorID},
			Position: pos,
			Body:     body,
		})
	}
	resolvedNote := func(n *gitlab.Note) *gitlab.Note {
		n.Resolved = true
		return n
	}
	systemNote := func(n *gitlab.Note) *gitlab.Note {
		n.System = true
		return n
	}

	testCases := []struct {
		mock           httpmock.Mocker
		errorHandler   errorCheck
		description    string
		summary        reporter.Summary
		timeout        time.Duration
		maxComments    int
		showDuplicates bool
	}{
		{
			description: "bogus JSON responses",
			timeout:     time.Minute,
			maxComments: 1,
			summary:     reporter.NewSummary([]reporter.Report{fooReport}),
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(apiUser).ReturnJSON(gitlab.User{ID: 123})
				s.ExpectGet(regexp.MustCompile(".+")).Times(4).Return(`[{"iid": 333}]`)
			}),
			errorHandler: func(err error) error {
				return err
			},
		},
		{
			description: "empty list of merge requests",
			timeout:     time.Minute,
			maxComments: 50,
			summary:     reporter.NewSummary([]reporter.Report{fooReport}),
			errorHandler: func(err error) error {
				return err
			},
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(apiUser).ReturnJSON(gitlab.User{ID: 123})
				s.ExpectGet(apiOpenMergeRequests).ReturnJSON([]gitlab.BasicMergeRequest{})
			}),
		},
		{
			description: "single MR with diagnostics",
			timeout:     time.Minute,
			maxComments: 1,
			summary: reporter.NewSummary([]reporter.Report{
				{
					Path: discovery.Path{
						SymlinkTarget: mockPath,
						Name:          mockPath,
					},
					ModifiedLines: []int{2},
					Rule:          mockFile.Groups[0].Rules[0],
					Problem: checks.Problem{
						Reporter: "a",
						Summary:  "foo error1",
						Details:  "foo details",
						Diagnostics: []diags.Diagnostic{
							{
								Message: "Diagnostic message",
								Pos: diags.PositionRanges{
									{
										Line:        2,
										FirstColumn: 3,
										LastColumn:  24,
									},
									{
										Line:        3,
										FirstColumn: 3,
										LastColumn:  15,
									},
								},
								FirstColumn: 1,
								LastColumn:  24,
								Kind:        diags.Issue,
							},
						},
						Lines:    diags.LineRange{First: 1, Last: 3},
						Severity: checks.Fatal,
						Anchor:   checks.AnchorAfter,
					},
				},
			}),
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(apiUser).ReturnJSON(gitlab.User{ID: 123})
				s.ExpectGet(apiOpenMergeRequests).ReturnJSON([]gitlab.BasicMergeRequest{
					{IID: 1},
				})
				s.ExpectGet(apiVersions(1)).ReturnJSON([]gitlab.MergeRequestDiffVersion{
					{ID: 2, HeadCommitSHA: "head", BaseCommitSHA: "base", StartCommitSHA: "start"},
					{ID: 1, HeadCommitSHA: "head", BaseCommitSHA: "base", StartCommitSHA: "start"},
				})
				s.ExpectGet(apiDiffs(1)).ReturnJSON([]gitlab.MergeRequestDiff{
					{OldPath: mockPath, NewPath: mockPath, Diff: fooDiff},
				})
				s.ExpectGet(apiDiscussions(1, true)).ReturnJSON([]gitlab.Discussion{})
				s.ExpectPost(apiDiscussions(1, false)).WithBodyJSON(gitlab.CreateMergeRequestDiscussionOptions{
					Body:     discBodyWithDiag("a", "foo error1", "foo details", "```yaml\n2 | - record: target is down\n3 |   expr: up == 0\n      ^^ \n```", "Diagnostic message"),
					Position: discPosition(mockPath, 2),
				}).ReturnJSON(gitlab.Response{})
			}),
			errorHandler: func(err error) error {
				return err
			},
		},
		{
			description: "multiple merge requests",
			timeout:     time.Minute,
			maxComments: 50,
			summary:     reporter.NewSummary([]reporter.Report{fooReport}),
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(apiUser).ReturnJSON(gitlab.User{ID: 123})
				s.ExpectGet(apiOpenMergeRequests).ReturnJSON([]gitlab.BasicMergeRequest{
					{IID: 1},
					{IID: 2},
					{IID: 5},
				})
				for _, i := range []int{1, 2, 5} {
					s.ExpectGet(apiVersions(i)).ReturnJSON([]gitlab.MergeRequestDiffVersion{
						{ID: 2, HeadCommitSHA: "head", BaseCommitSHA: "base", StartCommitSHA: "start"},
						{ID: 1, HeadCommitSHA: "head", BaseCommitSHA: "base", StartCommitSHA: "start"},
					})
					s.ExpectGet(apiDiffs(i)).ReturnJSON([]gitlab.MergeRequestDiff{
						{OldPath: "foo.txt", NewPath: "foo.txt", Diff: fooDiff},
					})
					if i == 5 {
						s.ExpectGet(apiDiscussions(i, true)).ReturnJSON([]gitlab.Discussion{})
					} else {
						s.ExpectGet(apiDiscussions(i, true)).ReturnJSON([]gitlab.Discussion{
							{ID: "100", Notes: []*gitlab.Note{systemNote(discNote(101, 123, "system message", nil))}},
							{ID: "200", Notes: []*gitlab.Note{discNote(201, 321, "different user", notePos("foo.txt", "foo.txt", 2, 0))}},
							{ID: "300", Notes: []*gitlab.Note{discNote(301, 123, "stale comment", notePos("foo.txt", "foo.txt", 2, 0))}},
							{ID: "400", Notes: []*gitlab.Note{discNote(401, 123, "stale comment on unmodified line", notePos("foo.txt", "foo.txt", 1, 0))}},
							{ID: "500", Notes: []*gitlab.Note{discNote(101, 123, "no position", nil)}},
						})
					}

				}

				for _, i := range []int{1, 2} {
					s.ExpectPost(apiDiscussions(i, false)).WithBodyJSON(gitlab.CreateMergeRequestDiscussionOptions{
						Body:     discBody("foo", "foo error", "foo details"),
						Position: discPosition("foo.txt", 2),
					}).ReturnJSON(gitlab.Response{})

					s.ExpectPut(apiDiscussions(i, false) + "/300").WithBodyJSON(
						gitlab.ResolveMergeRequestDiscussionOptions{Resolved: gitlab.Ptr(true)},
					).ReturnJSON(gitlab.Response{})
					s.ExpectPut(apiDiscussions(i, false) + "/400").WithBodyJSON(
						gitlab.ResolveMergeRequestDiscussionOptions{Resolved: gitlab.Ptr(true)},
					).ReturnJSON(gitlab.Response{})
				}

				s.ExpectPost(apiDiscussions(5, false)).WithBodyJSON(gitlab.CreateMergeRequestDiscussionOptions{
					Body:     gitlab.Ptr(":stop_sign: **Fatal** reported by [pint](https://cloudflare.github.io/pint/) **foo** check.\n\n------\n\nfoo error\n\n\u003cdetails\u003e\n\u003csummary\u003eMore information\u003c/summary\u003e\nfoo details\n\u003c/details\u003e\n\n------\n\n:information_source: To see documentation covering this check and instructions on how to resolve it [click here](https://cloudflare.github.io/pint/checks/foo.html).\n"),
					Position: discPosition("foo.txt", 2),
				}).ReturnJSON(gitlab.Response{})
			}),
			errorHandler: func(err error) error {
				return err
			},
		},
		{
			description: "list merge requests failed",
			timeout:     time.Minute,
			maxComments: 50,
			summary:     reporter.NewSummary([]reporter.Report{fooReport}),
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(apiUser).ReturnJSON(gitlab.User{ID: 123})
				s.ExpectGet(apiOpenMergeRequests).Times(6).
					ReturnCode(http.StatusInternalServerError).
					Return("Mock error")
			}),
			errorHandler: func(err error) error {
				if strings.HasSuffix(err.Error(), ": Mock error") {
					return nil
				}
				return err
			},
		},
		{
			description: "user request failed",
			timeout:     time.Minute,
			maxComments: 50,
			summary:     reporter.NewSummary([]reporter.Report{fooReport}),
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(apiUser).Times(6).
					ReturnCode(http.StatusInternalServerError).
					Return("Mock error")
			}),
			errorHandler: func(err error) error {
				if strings.HasSuffix(err.Error(), ": Mock error") {
					return nil
				}
				return err
			},
		},
		{
			description: "too many comments",
			timeout:     time.Minute,
			maxComments: 1,
			summary:     summaryABC,
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(apiUser).ReturnJSON(gitlab.User{ID: 123})
				s.ExpectGet(apiOpenMergeRequests).ReturnJSON([]gitlab.BasicMergeRequest{
					{IID: 1},
				})
				s.ExpectGet(apiVersions(1)).ReturnJSON([]gitlab.MergeRequestDiffVersion{
					{ID: 2, HeadCommitSHA: "head", BaseCommitSHA: "base", StartCommitSHA: "start"},
					{ID: 1, HeadCommitSHA: "head", BaseCommitSHA: "base", StartCommitSHA: "start"},
				})
				s.ExpectGet(apiDiffs(1)).ReturnJSON([]gitlab.MergeRequestDiff{
					{OldPath: "foo.txt", NewPath: "foo.txt", Diff: fooDiff},
				})
				s.ExpectGet(apiDiscussions(1, true)).ReturnJSON([]gitlab.Discussion{
					{
						ID:    "100",
						Notes: []*gitlab.Note{systemNote(discNote(101, 123, "system comment", nil))},
					},
					{
						ID:    "200",
						Notes: []*gitlab.Note{discNote(201, 321, "different user", nil)},
					},
					{
						ID:    "300",
						Notes: []*gitlab.Note{resolvedNote(discNote(301, 123, "different line", notePos("foo.txt", "foo.txt", 1, 0)))},
					},
				})
				s.ExpectPost(apiDiscussions(1, false)).WithBodyJSON(gitlab.CreateMergeRequestDiscussionOptions{
					Body:     discBody("a", "foo error1", "foo details"),
					Position: discPosition("foo.txt", 1),
				}).ReturnJSON(gitlab.Response{})
				s.ExpectPost(apiDiscussions(1, false)).WithBodyJSON(gitlab.CreateMergeRequestDiscussionOptions{
					Body: gitlab.Ptr("This pint run would create 3 comment(s), which is more than the limit configured for pint (1).\n2 comment(s) were skipped and won't be visibile on this PR."),
				}).ReturnJSON(gitlab.Response{})
			}),
			errorHandler: func(err error) error {
				return err
			},
		},
		{
			description: "no diff",
			timeout:     time.Minute,
			maxComments: 1,
			summary:     reporter.NewSummary([]reporter.Report{fooReport}),
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(apiUser).ReturnJSON(gitlab.User{ID: 123})
				s.ExpectGet(apiOpenMergeRequests).ReturnJSON([]gitlab.BasicMergeRequest{
					{IID: 1},
				})
				s.ExpectGet(apiVersions(1)).ReturnJSON([]gitlab.MergeRequestDiffVersion{
					{ID: 2, HeadCommitSHA: "head", BaseCommitSHA: "base", StartCommitSHA: "start"},
					{ID: 1, HeadCommitSHA: "head", BaseCommitSHA: "base", StartCommitSHA: "start"},
				})
				s.ExpectGet(apiDiffs(1)).ReturnJSON([]gitlab.MergeRequestDiff{})
				s.ExpectGet(apiDiscussions(1, true)).ReturnJSON([]gitlab.Discussion{})
			}),
			errorHandler: func(err error) error {
				return err
			},
		},
		{
			description: "diff error",
			timeout:     time.Minute,
			maxComments: 1,
			summary:     reporter.NewSummary([]reporter.Report{fooReport}),
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(apiUser).ReturnJSON(gitlab.User{ID: 123})
				s.ExpectGet(apiOpenMergeRequests).ReturnJSON([]gitlab.BasicMergeRequest{
					{IID: 1},
				})
				s.ExpectGet(apiVersions(1)).ReturnJSON([]gitlab.MergeRequestDiffVersion{
					{ID: 2, HeadCommitSHA: "head", BaseCommitSHA: "base", StartCommitSHA: "start"},
					{ID: 1, HeadCommitSHA: "head", BaseCommitSHA: "base", StartCommitSHA: "start"},
				})
				s.ExpectGet(apiDiffs(1)).
					ReturnCode(http.StatusForbidden).
					ReturnJSON(map[string]string{"error": "Mock error"})
			}),
			errorHandler: func(err error) error {
				if strings.HasSuffix(err.Error(), ": 403 {error: Mock error}") {
					return nil
				}
				return err
			},
		},
		{
			description: "no versions",
			timeout:     time.Minute,
			maxComments: 1,
			summary:     reporter.NewSummary([]reporter.Report{fooReport}),
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(apiUser).ReturnJSON(gitlab.User{ID: 123})
				s.ExpectGet(apiOpenMergeRequests).ReturnJSON([]gitlab.BasicMergeRequest{
					{IID: 1},
				})
				s.ExpectGet(apiVersions(1)).ReturnJSON([]gitlab.MergeRequestDiffVersion{})
			}),
			errorHandler: func(err error) error {
				if strings.HasSuffix(err.Error(), ": no merge request versions found") {
					return nil
				}
				return err
			},
		},
		{
			description: "versions request error",
			timeout:     time.Minute,
			maxComments: 1,
			summary:     reporter.NewSummary([]reporter.Report{fooReport}),
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(apiUser).ReturnJSON(gitlab.User{ID: 123})
				s.ExpectGet(apiOpenMergeRequests).ReturnJSON([]gitlab.BasicMergeRequest{
					{IID: 1},
				})
				s.ExpectGet(apiVersions(1)).
					Times(6).
					ReturnCode(http.StatusBadGateway).
					ReturnJSON(map[string]string{"error": "Mock error"})
			}),
			errorHandler: func(err error) error {
				if strings.HasSuffix(err.Error(), ": 502 {error: Mock error}") {
					return nil
				}
				return err
			},
		},
		{
			description: "disabled checks",
			timeout:     time.Minute,
			maxComments: 10,
			summary:     summaryWithDetails,
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(apiUser).ReturnJSON(gitlab.User{ID: 123})
				s.ExpectGet(apiOpenMergeRequests).ReturnJSON([]gitlab.BasicMergeRequest{
					{IID: 1},
				})
				s.ExpectGet(apiVersions(1)).ReturnJSON([]gitlab.MergeRequestDiffVersion{
					{ID: 2, HeadCommitSHA: "head", BaseCommitSHA: "base", StartCommitSHA: "start"},
					{ID: 1, HeadCommitSHA: "head", BaseCommitSHA: "base", StartCommitSHA: "start"},
				})
				s.ExpectGet(apiDiffs(1)).ReturnJSON([]gitlab.MergeRequestDiff{
					{OldPath: "foo.txt", NewPath: "foo.txt", Diff: fooDiff},
				})
				s.ExpectGet(apiDiscussions(1, true)).ReturnJSON([]gitlab.Discussion{})
				s.ExpectPost(apiDiscussions(1, false)).WithBodyJSON(gitlab.CreateMergeRequestDiscussionOptions{
					Body: gitlab.Ptr(`Some checks were disabled because one or more configured Prometheus server doesn't seem to support all required Prometheus APIs.
This usually means that you're running pint against a service like Thanos or Mimir that allows to query metrics but doesn't implement all APIs documented [here](https://prometheus.io/docs/prometheus/latest/querying/api/).
Since pint uses many of these API endpoint for querying information needed to run online checks only a real Prometheus server will allow it to run all of these checks.
Below is the list of checks that were disabled for each Prometheus server defined in pint config file.

- ` + "`prom1`" + `
  - ` + "`/api/v1/status/config` " + `is unsupported, disabled checks:
    - [check1](https://cloudflare.github.io/pint/checks/check1.html)
    - [check2](https://cloudflare.github.io/pint/checks/check2.html)
    - [check3](https://cloudflare.github.io/pint/checks/check3.html)
- ` + "`prom2`" + `
  - ` + "`/api/v1/metadata` " + `is unsupported, disabled checks:
    - [check1](https://cloudflare.github.io/pint/checks/check1.html)
`),
				}).ReturnJSON(gitlab.Response{})
			}),
			errorHandler: func(err error) error {
				return err
			},
		},
		{
			description: "disabled checks / error",
			timeout:     time.Minute,
			maxComments: 10,
			summary:     summaryWithDetails,
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(apiUser).ReturnJSON(gitlab.User{ID: 123})
				s.ExpectGet(apiOpenMergeRequests).ReturnJSON([]gitlab.BasicMergeRequest{
					{IID: 1},
				})
				s.ExpectGet(apiVersions(1)).ReturnJSON([]gitlab.MergeRequestDiffVersion{
					{ID: 2, HeadCommitSHA: "head", BaseCommitSHA: "base", StartCommitSHA: "start"},
					{ID: 1, HeadCommitSHA: "head", BaseCommitSHA: "base", StartCommitSHA: "start"},
				})
				s.ExpectGet(apiDiffs(1)).ReturnJSON([]gitlab.MergeRequestDiff{
					{OldPath: "foo.txt", NewPath: "foo.txt", Diff: fooDiff},
				})
				s.ExpectGet(apiDiscussions(1, true)).ReturnJSON([]gitlab.Discussion{})
				s.ExpectPost(apiDiscussions(1, false)).
					ReturnCode(http.StatusBadRequest).
					ReturnJSON(map[string]string{"error": "Mock error"})
			}),
			errorHandler: func(err error) error {
				if strings.HasSuffix(err.Error(), ": 400 {error: Mock error}") {
					return nil
				}
				return err
			},
		},
		{
			description: "general comment already present",
			timeout:     time.Minute,
			maxComments: 1,
			summary:     summaryABC,
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(apiUser).ReturnJSON(gitlab.User{ID: 123})
				s.ExpectGet(apiOpenMergeRequests).ReturnJSON([]gitlab.BasicMergeRequest{
					{IID: 1},
				})
				s.ExpectGet(apiVersions(1)).ReturnJSON([]gitlab.MergeRequestDiffVersion{
					{ID: 2, HeadCommitSHA: "head", BaseCommitSHA: "base", StartCommitSHA: "start"},
					{ID: 1, HeadCommitSHA: "head", BaseCommitSHA: "base", StartCommitSHA: "start"},
				})
				s.ExpectGet(apiDiffs(1)).ReturnJSON([]gitlab.MergeRequestDiff{
					{OldPath: "foo.txt", NewPath: "foo.txt", Diff: fooDiff},
				})
				s.ExpectGet(apiDiscussions(1, true)).ReturnJSON([]gitlab.Discussion{
					{
						ID:    "100",
						Notes: []*gitlab.Note{systemNote(discNote(101, 123, "system comment", nil))},
					},
					{
						ID:    "200",
						Notes: []*gitlab.Note{discNote(201, 321, "different user", nil)},
					},
					{
						ID:    "300",
						Notes: []*gitlab.Note{discNote(301, 123, "This pint run would create 3 comment(s), which is more than the limit configured for pint (1).\n2 comment(s) were skipped and won't be visibile on this PR.", nil)},
					},
				})
				s.ExpectPost(apiDiscussions(1, false)).WithBodyJSON(gitlab.CreateMergeRequestDiscussionOptions{
					Body:     discBody("a", "foo error1", "foo details"),
					Position: discPosition("foo.txt", 1),
				}).ReturnJSON(gitlab.Response{})
			}),
			errorHandler: func(err error) error {
				return err
			},
		},
		{
			description: "general comment already present / error",
			timeout:     time.Minute,
			maxComments: 1,
			summary:     summaryABC,
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(apiUser).ReturnJSON(gitlab.User{ID: 123})
				s.ExpectGet(apiOpenMergeRequests).ReturnJSON([]gitlab.BasicMergeRequest{
					{IID: 1},
				})
				s.ExpectGet(apiVersions(1)).ReturnJSON([]gitlab.MergeRequestDiffVersion{
					{ID: 2, HeadCommitSHA: "head", BaseCommitSHA: "base", StartCommitSHA: "start"},
					{ID: 1, HeadCommitSHA: "head", BaseCommitSHA: "base", StartCommitSHA: "start"},
				})
				s.ExpectGet(apiDiffs(1)).ReturnJSON([]gitlab.MergeRequestDiff{
					{OldPath: "foo.txt", NewPath: "foo.txt", Diff: fooDiff},
				})
				// Problem comment
				s.ExpectGet(apiDiscussions(1, true)).ReturnJSON([]gitlab.Discussion{})
				s.ExpectPost(apiDiscussions(1, false)).WithBodyJSON(gitlab.CreateMergeRequestDiscussionOptions{
					Body:     discBody("a", "foo error1", "foo details"),
					Position: discPosition("foo.txt", 1),
				}).ReturnJSON(gitlab.Response{})
				// Too many comments comment
				s.ExpectPost(apiDiscussions(1, false)).
					Times(2).
					ReturnCode(http.StatusBadRequest).
					ReturnJSON(map[string]string{"error": "Mock error"})
			}),
			errorHandler: func(err error) error {
				if strings.HasSuffix(err.Error(), ": 400 {error: Mock error}") {
					return nil
				}
				return err
			},
		},
		{
			description: "list discussions failed",
			timeout:     time.Minute,
			maxComments: 1,
			summary:     summaryABC,
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(apiUser).ReturnJSON(gitlab.User{ID: 123})
				s.ExpectGet(apiOpenMergeRequests).ReturnJSON([]gitlab.BasicMergeRequest{
					{IID: 1},
				})
				s.ExpectGet(apiVersions(1)).ReturnJSON([]gitlab.MergeRequestDiffVersion{
					{ID: 2, HeadCommitSHA: "head", BaseCommitSHA: "base", StartCommitSHA: "start"},
					{ID: 1, HeadCommitSHA: "head", BaseCommitSHA: "base", StartCommitSHA: "start"},
				})
				s.ExpectGet(apiDiffs(1)).ReturnJSON([]gitlab.MergeRequestDiff{
					{OldPath: "foo.txt", NewPath: "foo.txt", Diff: fooDiff},
				})
				// Problem comment
				s.ExpectGet(apiDiscussions(1, true)).
					ReturnCode(http.StatusBadRequest).
					ReturnJSON(map[string]string{"error": "Mock error"})
			}),
			errorHandler: func(err error) error {
				if strings.HasSuffix(err.Error(), ": 400 {error: Mock error}") {
					return nil
				}
				return err
			},
		},
		{
			description: "comments already exist",
			timeout:     time.Minute,
			maxComments: 10,
			summary:     summaryABC,
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(apiUser).ReturnJSON(gitlab.User{ID: 123})
				s.ExpectGet(apiOpenMergeRequests).ReturnJSON([]gitlab.BasicMergeRequest{
					{IID: 1},
				})
				s.ExpectGet(apiVersions(1)).ReturnJSON([]gitlab.MergeRequestDiffVersion{
					{ID: 2, HeadCommitSHA: "head", BaseCommitSHA: "base", StartCommitSHA: "start"},
					{ID: 1, HeadCommitSHA: "head", BaseCommitSHA: "base", StartCommitSHA: "start"},
				})
				s.ExpectGet(apiDiffs(1)).ReturnJSON([]gitlab.MergeRequestDiff{
					{OldPath: "foo.txt", NewPath: "foo.txt", Diff: fooDiff},
				})
				s.ExpectGet(apiDiscussions(1, true)).ReturnJSON([]gitlab.Discussion{
					{
						ID: "100",
						Notes: []*gitlab.Note{
							systemNote(discNote(101, 321, "system note", nil)),
						},
					},
					{
						ID: "200",
						Notes: []*gitlab.Note{
							discNote(201, 123, "different path", notePos("bar.txt", "bar.txt", 5, 0)),
						},
					},
					{
						ID: "300",
						Notes: []*gitlab.Note{
							discNote(301, 123, "different path", notePos("bar.txt", "", 1, 0)),
						},
					},
					{
						ID: "400",
						Notes: []*gitlab.Note{
							discNote(401, 123, "different line", notePos("foo.txt", "", 0, 1)),
						},
					},
					{
						ID: "500",
						Notes: []*gitlab.Note{
							discNote(501, 123, *discBody("a", "foo error1", "foo details"), notePos("foo.txt", "foo.txt", 1, 0)),
						},
					},
					{
						ID: "600",
						Notes: []*gitlab.Note{
							discNote(601, 123, *discBody("b", "foo error2", "foo details"), notePos("foo.txt", "foo.txt", 2, 0)),
						},
					},
					{
						ID: "700",
						Notes: []*gitlab.Note{
							discNote(701, 123, *discBody("c", "foo error3", "foo details"), notePos("foo.txt", "foo.txt", 3, 0)),
						},
					},
				})
				s.ExpectPut(apiDiscussions(1, false) + "/200").WithBodyJSON(
					gitlab.ResolveMergeRequestDiscussionOptions{Resolved: gitlab.Ptr(true)},
				).ReturnJSON(gitlab.Response{})
				s.ExpectPut(apiDiscussions(1, false) + "/300").WithBodyJSON(
					gitlab.ResolveMergeRequestDiscussionOptions{Resolved: gitlab.Ptr(true)},
				).ReturnJSON(gitlab.Response{})
				s.ExpectPut(apiDiscussions(1, false) + "/400").WithBodyJSON(
					gitlab.ResolveMergeRequestDiscussionOptions{Resolved: gitlab.Ptr(true)},
				).ReturnJSON(gitlab.Response{})
			}),
			errorHandler: func(err error) error {
				return err
			},
		},
		{
			description: "comments already exist but resolved",
			timeout:     time.Minute,
			maxComments: 10,
			summary:     summaryABC,
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(apiUser).ReturnJSON(gitlab.User{ID: 123})
				s.ExpectGet(apiOpenMergeRequests).ReturnJSON([]gitlab.BasicMergeRequest{
					{IID: 1},
				})
				s.ExpectGet(apiVersions(1)).ReturnJSON([]gitlab.MergeRequestDiffVersion{
					{ID: 2, HeadCommitSHA: "head", BaseCommitSHA: "base", StartCommitSHA: "start"},
					{ID: 1, HeadCommitSHA: "head", BaseCommitSHA: "base", StartCommitSHA: "start"},
				})
				s.ExpectGet(apiDiffs(1)).ReturnJSON([]gitlab.MergeRequestDiff{
					{OldPath: "foo.txt", NewPath: "foo.txt", Diff: fooDiff},
				})
				s.ExpectGet(apiDiscussions(1, true)).ReturnJSON([]gitlab.Discussion{
					{
						ID: "100",
						Notes: []*gitlab.Note{
							discNote(101, 123, *discBody("a", "foo error1", "foo details"), notePos("foo.txt", "foo.txt", 1, 0)),
						},
					},
					{
						ID: "200",
						Notes: []*gitlab.Note{
							resolvedNote(
								discNote(201, 123, *discBody("b", "foo error2", "foo details"), notePos("foo.txt", "foo.txt", 2, 0)),
							),
						},
					},
					{
						ID: "300",
						Notes: []*gitlab.Note{
							discNote(301, 123, *discBody("c", "foo error3", "foo details"), notePos("foo.txt", "foo.txt", 3, 0)),
						},
					},
				})
				s.ExpectPut(apiDiscussions(1, false) + "/200").WithBodyJSON(
					gitlab.ResolveMergeRequestDiscussionOptions{Resolved: gitlab.Ptr(false)},
				).ReturnJSON(gitlab.Response{})
			}),
			errorHandler: func(err error) error {
				return err
			},
		},
		{
			description: "comments already exist but resolved / resolve error",
			timeout:     time.Minute,
			maxComments: 10,
			summary:     summaryABC,
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(apiUser).ReturnJSON(gitlab.User{ID: 123})
				s.ExpectGet(apiOpenMergeRequests).ReturnJSON([]gitlab.BasicMergeRequest{
					{IID: 1},
				})
				s.ExpectGet(apiVersions(1)).ReturnJSON([]gitlab.MergeRequestDiffVersion{
					{ID: 2, HeadCommitSHA: "head", BaseCommitSHA: "base", StartCommitSHA: "start"},
					{ID: 1, HeadCommitSHA: "head", BaseCommitSHA: "base", StartCommitSHA: "start"},
				})
				s.ExpectGet(apiDiffs(1)).ReturnJSON([]gitlab.MergeRequestDiff{
					{OldPath: "foo.txt", NewPath: "foo.txt", Diff: fooDiff},
				})
				s.ExpectGet(apiDiscussions(1, true)).ReturnJSON([]gitlab.Discussion{
					{
						ID: "100",
						Notes: []*gitlab.Note{
							discNote(101, 123, *discBody("a", "foo error1", "foo details"), notePos("foo.txt", "foo.txt", 1, 0)),
						},
					},
					{
						ID: "200",
						Notes: []*gitlab.Note{
							resolvedNote(
								discNote(201, 123, *discBody("b", "foo error2", "foo details"), notePos("foo.txt", "foo.txt", 2, 0)),
							),
						},
					},
					{
						ID: "300",
						Notes: []*gitlab.Note{
							discNote(301, 123, *discBody("c", "foo error3", "foo details"), notePos("foo.txt", "foo.txt", 3, 0)),
						},
					},
				})
				s.ExpectPut(apiDiscussions(1, false) + "/200").WithBodyJSON(
					gitlab.ResolveMergeRequestDiscussionOptions{Resolved: gitlab.Ptr(false)},
				).Times(6).ReturnCode(http.StatusInternalServerError).Return("Mock error")
				s.ExpectPost(apiDiscussions(1, false)).WithBodyJSON(gitlab.CreateMergeRequestDiscussionOptions{
					Body:     discBody("b", "foo error2", "foo details"),
					Position: discPosition("foo.txt", 2),
				}).ReturnJSON(gitlab.Response{})
			}),
			errorHandler: func(err error) error {
				return err
			},
		},
		{
			description: "rule moved to a different file, old deleted",
			timeout:     time.Minute,
			maxComments: 1,
			summary: reporter.NewSummary([]reporter.Report{
				{
					Path: discovery.Path{
						SymlinkTarget: mockPath,
						Name:          mockPath,
					},
					ModifiedLines: []int{2, 3},
					Rule:          mockFile.Groups[0].Rules[0],
					Problem: checks.Problem{
						Reporter: "a",
						Summary:  "foo error1",
						Details:  "foo details",
						Diagnostics: []diags.Diagnostic{
							{
								Message: "Diagnostic message",
								Pos: diags.PositionRanges{
									{
										Line:        2,
										FirstColumn: 3,
										LastColumn:  24,
									},
									{
										Line:        3,
										FirstColumn: 3,
										LastColumn:  15,
									},
								},
								FirstColumn: 1,
								LastColumn:  24,
								Kind:        diags.Issue,
							},
						},
						Lines:    diags.LineRange{First: 1, Last: 3},
						Severity: checks.Fatal,
						Anchor:   checks.AnchorAfter,
					},
				},
			}),
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(apiUser).ReturnJSON(gitlab.User{ID: 123})
				s.ExpectGet(apiOpenMergeRequests).ReturnJSON([]gitlab.BasicMergeRequest{
					{IID: 1},
				})
				s.ExpectGet(apiVersions(1)).ReturnJSON([]gitlab.MergeRequestDiffVersion{
					{ID: 2, HeadCommitSHA: "head", BaseCommitSHA: "base", StartCommitSHA: "start"},
					{ID: 1, HeadCommitSHA: "head", BaseCommitSHA: "base", StartCommitSHA: "start"},
				})
				s.ExpectGet(apiDiffs(1)).ReturnJSON([]gitlab.MergeRequestDiff{
					{
						Diff:        "",
						NewPath:     mockPath,
						OldPath:     "foo.old",
						AMode:       "100644",
						BMode:       "100644",
						RenamedFile: true,
					},
					{
						Diff:    fooDiff,
						NewPath: "foo.txt",
						OldPath: "foo.txt",
					},
				})
				s.ExpectGet(apiDiscussions(1, true)).ReturnJSON([]gitlab.Discussion{})
				s.ExpectPost(apiDiscussions(1, false)).WithBodyJSON(gitlab.CreateMergeRequestDiscussionOptions{
					Body: discBodyWithDiag("a", "foo error1", "foo details", "```yaml\n2 | - record: target is down\n3 |   expr: up == 0\n      ^^ \n```", "Diagnostic message"),
					Position: gitlab.Ptr(gitlab.PositionOptions{
						BaseSHA:      gitlab.Ptr("base"),
						StartSHA:     gitlab.Ptr("start"),
						HeadSHA:      gitlab.Ptr("head"),
						OldPath:      gitlab.Ptr("foo.old"),
						NewPath:      gitlab.Ptr(mockPath),
						PositionType: gitlab.Ptr("text"),
						NewLine:      gitlab.Ptr(3),
						// Old file is gone so we don't have OldLine here at all
					}),
				}).ReturnJSON(gitlab.Response{})
			}),
			errorHandler: func(err error) error {
				return err
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			slog.SetDefault(slogt.New(t))
			srv := tc.mock(t)
			t.Cleanup(srv.Close)
			r, err := reporter.NewGitLabReporter(
				"v0.0.0",
				"fakeBranch",
				srv.URL(),
				tc.timeout,
				"fakeToken",
				123,
				tc.maxComments,
			)
			if err == nil {
				err = reporter.Submit(t.Context(), tc.summary, r, tc.showDuplicates)
				require.NoError(t, tc.errorHandler(err))
			}
			require.NoError(t, tc.errorHandler(err))
		})
	}
}

func TestGitLabReporterCommentLine(t *testing.T) {
	type testCaseT struct {
		description     string
		problemLine     int
		expectedNewLine int
		expectedOldLine int
		anchor          checks.Anchor
		showDuplicates  bool
	}

	p := parser.NewParser(false, parser.PrometheusSchema, model.UTF8Validation)
	mockFile := p.Parse(strings.NewReader(`
- record: target is down
  expr: up == 0
- record: sum errors
  expr: sum(errors) by (job)
`))

	multipleDiffs := `@@ -15,6 +15,7 @@ spec:
      annotations:
	    foo: bar
        description: some description
+       runbook_url: https://runbook.url
        summary: summary text
      expr: up == 0
      for: 10m
@@ -141,6 +142,5 @@ spec:
        description: some description
        summary: some summary
      expr: sum(errors) by (job)
-     for: 15m
      labels:
        severity: warning`
	multipleDiffs = strings.ReplaceAll(multipleDiffs, "\n", "\\n")
	multipleDiffs = strings.ReplaceAll(multipleDiffs, "\t", "\\t")

	errorHandler := func(err error) error { return err }

	testCases := []testCaseT{
		{
			description:     "comment on new line",
			problemLine:     18,
			expectedNewLine: 18,
		},
		{
			description:     "comment on removed line",
			problemLine:     145,
			expectedOldLine: 145,
			anchor:          checks.AnchorBefore,
		},
		{
			description:     "unmodified line before existing line in the diff",
			problemLine:     10,
			expectedNewLine: 10,
			expectedOldLine: 10,
		},
		{
			description:     "unmodified line that exists in the diff",
			problemLine:     16,
			expectedNewLine: 16,
			expectedOldLine: 16,
		},
		{
			description:     "unmodified line after added line and exists in the diff",
			problemLine:     19,
			expectedNewLine: 19,
			expectedOldLine: 18,
		},
		{
			description:     "unmodified line after added line and not exists in the diff",
			problemLine:     23,
			expectedNewLine: 23,
			expectedOldLine: 22,
		},
		{
			description:     "unmodified line before removed line and exists in the diff",
			problemLine:     143,
			expectedNewLine: 143,
			expectedOldLine: 142,
		},
		{
			description:     "unmodified line after removed line and exists in the diff",
			problemLine:     145,
			expectedNewLine: 145,
			expectedOldLine: 145,
		},
		{
			description:     "unmodified line after removed line and not exists in the diff",
			problemLine:     148,
			expectedNewLine: 148,
			expectedOldLine: 148,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			slog.SetDefault(slogt.New(t))

			srv := httptest.NewServer(getHTTPHandlerForCommentingLines(
				tc.expectedNewLine, tc.expectedOldLine, multipleDiffs, t))
			t.Cleanup(srv.Close)

			r, err := reporter.NewGitLabReporter(
				"v0.0.0",
				"fakeBranch",
				srv.URL,
				time.Minute,
				"fakeToken",
				123,
				10,
			)
			if err == nil {
				summary := reporter.NewSummary([]reporter.Report{
					{
						Path: discovery.Path{
							Name:          "foo.txt",
							SymlinkTarget: "foo.txt",
						},
						ModifiedLines: []int{2},
						Rule:          mockFile.Groups[0].Rules[1],
						Problem: checks.Problem{
							Lines: diags.LineRange{
								First: tc.problemLine,
								Last:  tc.problemLine,
							},
							Reporter: "mock",
							Summary:  "syntax error",
							Details:  "syntax details",
							Severity: checks.Fatal,
							Anchor:   tc.anchor,
						},
					},
				})
				err = reporter.Submit(t.Context(), summary, r, tc.showDuplicates)
			}
			require.NoError(t, errorHandler(err))
		})
	}
}

func getHTTPHandlerForCommentingLines(expectedNewLine, expectedOldLine int, diff string, t *testing.T) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		switch r.URL.Path {
		case "/api/v4/user":
			if r.Method == http.MethodGet {
				_, _ = w.Write([]byte(`{"id": 123}`))
			}
		case "/api/v4/projects/123/merge_requests":
			if r.Method == http.MethodGet {
				_, _ = w.Write([]byte(`[{"iid":1}]`))
			}
		case "/api/v4/projects/123/merge_requests/1/diffs":
			if r.Method == http.MethodGet {
				_, _ = w.Write([]byte(`[{"diff":"` + diff + `","new_path":"foo.txt","old_path":"foo.txt"}]`))
			}
		case "/api/v4/projects/123/merge_requests/1/versions":
			if r.Method == http.MethodGet {
				_, _ = w.Write([]byte(`[
	{"id": 2,"head_commit_sha": "head","base_commit_sha": "base","start_commit_sha": "start"},
	{"id": 1,"head_commit_sha": "head","base_commit_sha": "base","start_commit_sha": "start"}
	]`))
			}
		case "/api/v4/projects/123/merge_requests/1/discussions":
			if r.Method == http.MethodGet {
				_, _ = w.Write([]byte(`[]`))
			} else {
				body, _ := io.ReadAll(r.Body)
				b := strings.TrimSpace(strings.TrimRight(string(body), "\n\t\r"))

				str := ""
				if expectedNewLine != 0 {
					str = `"new_line":` + strconv.Itoa(expectedNewLine)
				}
				if expectedOldLine != 0 {
					if str != "" {
						str += ","
					}
					str += `"old_line":` + strconv.Itoa(expectedOldLine)
				}

				expected := `{
		"body":":stop_sign: **Fatal** reported by [pint](https://cloudflare.github.io/pint/) **mock** check.\n\n------\n\nsyntax error\n\n\u003cdetails\u003e\n\u003csummary\u003eMore information\u003c/summary\u003e\nsyntax details\n\u003c/details\u003e\n\n------\n\n:information_source: To see documentation covering this check and instructions on how to resolve it [click here](https://cloudflare.github.io/pint/checks/mock.html).\n",
		"position":{
			"base_sha":"base",
			"head_sha":"head",
			"start_sha":"start",
			"new_path":"foo.txt",
			"old_path":"foo.txt",
			"position_type":"text",
			` + str + `
		}
	}`
				expected = strings.ReplaceAll(expected, "\n", "")
				expected = strings.ReplaceAll(expected, "\t", "")
				if diff := cmp.Diff(b, expected); diff != "" {
					t.Errorf("Unexpected comment: (-want +got):\n%s", diff)
					t.FailNow()
				}
				_, _ = w.Write([]byte(`{}`))
			}
		}
	})
}
