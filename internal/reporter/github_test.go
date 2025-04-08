package reporter_test

import (
	"encoding/json"
	"fmt"
	"io"
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
	"github.com/cloudflare/pint/internal/promapi"
	"github.com/cloudflare/pint/internal/reporter"
)

func TestGitHubReporter(t *testing.T) {
	type testCaseT struct {
		httpHandler http.Handler
		error       func(uri string) string

		description string

		owner       string
		repo        string
		token       string
		summary     reporter.Summary
		prNum       int
		maxComments int
		timeout     time.Duration
	}

	p := parser.NewParser(false, parser.PrometheusSchema, model.UTF8Validation)
	mockFile := p.Parse(strings.NewReader(`
- record: target is down
  expr: up == 0
- record: sum errors
  expr: sum(errors) by (job)
`))

	summaryWithDetails := reporter.NewSummary([]reporter.Report{})
	summaryWithDetails.MarkCheckDisabled("prom1", promapi.APIPathConfig, []string{"check1", "check3", "check2"})
	summaryWithDetails.MarkCheckDisabled("prom2", promapi.APIPathMetadata, []string{"check1"})

	for _, tc := range []testCaseT{
		{
			description: "list files error",
			owner:       "foo",
			repo:        "bar",
			token:       "something",
			prNum:       123,
			maxComments: 50,
			timeout:     time.Second,
			httpHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet && r.URL.Path == "/api/v3/repos/foo/bar/pulls/123/files" {
					w.WriteHeader(http.StatusBadRequest)
					_, _ = w.Write([]byte("Error"))
					return
				}
				_, _ = w.Write([]byte(""))
			}),
			error: func(uri string) string {
				return fmt.Sprintf("failed to list pull request files: GET %s/api/v3/repos/foo/bar/pulls/123/files: 400  []", uri)
			},
			summary: reporter.NewSummary([]reporter.Report{
				{
					Path: discovery.Path{
						Name:          "foo.txt",
						SymlinkTarget: "foo.txt",
					},

					ModifiedLines: []int{2},
					Rule:          mockFile.Groups[0].Rules[1],
					Problem: checks.Problem{
						Lines: diags.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "mock",
						Summary:  "syntax error",
						Details:  "syntax details",
						Severity: checks.Fatal,
					},
				},
			}),
		},
		{
			description: "list pull reviews timeout",
			owner:       "foo",
			repo:        "bar",
			token:       "something",
			prNum:       123,
			maxComments: 50,
			httpHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet && r.URL.Path == "/api/v3/repos/foo/bar/pulls/123/files" {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("[]"))
					return
				}
				time.Sleep(1 * time.Second)
				_, _ = w.Write([]byte("OK"))
			}),
			timeout: 100 * time.Millisecond,
			error: func(_ string) string {
				return "failed to list pull request reviews: context deadline exceeded"
			},
			summary: reporter.NewSummary([]reporter.Report{
				{
					Path: discovery.Path{
						Name:          "$1",
						SymlinkTarget: "$1",
					},

					ModifiedLines: []int{2},
					Rule:          mockFile.Groups[0].Rules[1],
					Problem: checks.Problem{
						Lines: diags.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "mock",
						Summary:  "syntax error",
						Severity: checks.Fatal,
					},
				},
			}),
		},
		{
			description: "list reviews error",
			owner:       "foo",
			repo:        "bar",
			token:       "something",
			prNum:       123,
			maxComments: 50,
			timeout:     time.Second,
			httpHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet && r.URL.Path == "/api/v3/repos/foo/bar/pulls/123/reviews" {
					w.WriteHeader(http.StatusBadRequest)
					_, _ = w.Write([]byte("Error"))
					return
				}
				_, _ = w.Write([]byte(""))
			}),
			error: func(uri string) string {
				return fmt.Sprintf("failed to list pull request reviews: GET %s/api/v3/repos/foo/bar/pulls/123/reviews: 400  []", uri)
			},
			summary: reporter.NewSummary([]reporter.Report{
				{
					Path: discovery.Path{
						Name:          "foo.txt",
						SymlinkTarget: "foo.txt",
					},

					ModifiedLines: []int{2},
					Rule:          mockFile.Groups[0].Rules[1],
					Problem: checks.Problem{
						Lines: diags.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "mock",
						Summary:  "syntax error",
						Details:  "syntax details",
						Severity: checks.Fatal,
					},
				},
			}),
		},
		{
			description: "no problems",
			owner:       "foo",
			repo:        "bar",
			token:       "something",
			prNum:       123,
			maxComments: 50,
			timeout:     time.Second,
			summary:     reporter.NewSummary([]reporter.Report{}),
		},
		{
			description: "happy path",
			owner:       "foo",
			repo:        "bar",
			token:       "something",
			prNum:       123,
			maxComments: 50,
			timeout:     time.Second,
			summary: reporter.NewSummary([]reporter.Report{
				{
					Path: discovery.Path{
						Name:          "foo.txt",
						SymlinkTarget: "foo.txt",
					},

					ModifiedLines: []int{2},
					Rule:          mockFile.Groups[0].Rules[1],
					Problem: checks.Problem{
						Lines: diags.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "mock",
						Summary:  "syntax error",
						Details:  "syntax details",
						Severity: checks.Fatal,
					},
				},
			}),
		},
		{
			description: "error crating review",
			owner:       "foo",
			repo:        "bar",
			token:       "something",
			prNum:       123,
			maxComments: 50,
			timeout:     time.Second,
			httpHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost && r.URL.Path == "/api/v3/repos/foo/bar/pulls/123/reviews" {
					w.WriteHeader(http.StatusBadGateway)
					_, _ = w.Write([]byte("Error"))
					return
				}
				_, _ = w.Write([]byte(""))
			}),
			error: func(uri string) string {
				return fmt.Sprintf("failed to create pull request review: POST %s/api/v3/repos/foo/bar/pulls/123/reviews: 502  []", uri)
			},
			summary: reporter.NewSummary([]reporter.Report{
				{
					Path: discovery.Path{
						Name:          "foo.txt",
						SymlinkTarget: "foo.txt",
					},

					ModifiedLines: []int{2},
					Rule:          mockFile.Groups[0].Rules[1],
					Problem: checks.Problem{
						Lines: diags.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "mock",
						Summary:  "syntax error",
						Details:  "syntax details",
						Severity: checks.Fatal,
					},
				},
			}),
		},
		{
			description: "error updating existing review",
			owner:       "foo",
			repo:        "bar",
			token:       "something",
			prNum:       123,
			maxComments: 50,
			timeout:     time.Second,
			httpHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet && r.URL.Path == "/api/v3/repos/foo/bar/pulls/123/reviews" {
					_, _ = w.Write([]byte(`[{"id":1,"body":"### This pull request was validated by [pint](https://github.com/cloudflare/pint).\nxxxx"}]`))
					return
				}
				if r.Method == http.MethodPut && r.URL.Path == "/api/v3/repos/foo/bar/pulls/123/reviews/1" {
					w.WriteHeader(http.StatusBadGateway)
					_, _ = w.Write([]byte("Error"))
					return
				}
				_, _ = w.Write([]byte(""))
			}),
			error: func(uri string) string {
				return fmt.Sprintf("failed to update pull request review: PUT %s/api/v3/repos/foo/bar/pulls/123/reviews/1: 502  []", uri)
			},
			summary: reporter.NewSummary([]reporter.Report{
				{
					Path: discovery.Path{
						Name:          "foo.txt",
						SymlinkTarget: "foo.txt",
					},

					ModifiedLines: []int{2},
					Rule:          mockFile.Groups[0].Rules[1],
					Problem: checks.Problem{
						Lines: diags.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "mock",
						Summary:  "syntax error",
						Details:  "syntax details",
						Severity: checks.Fatal,
					},
				},
			}),
		},
		{
			description: "update existing review",
			owner:       "foo",
			repo:        "bar",
			token:       "something",
			prNum:       123,
			maxComments: 50,
			timeout:     time.Second,
			httpHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet && r.URL.Path == "/api/v3/repos/foo/bar/pulls/123/reviews" {
					_, _ = w.Write([]byte(`[{"id":1,"body":"### This pull request was validated by [pint](https://github.com/cloudflare/pint).\nxxxx"}]`))
					return
				}
				if r.Method == http.MethodGet && r.URL.Path == "/api/v3/repos/foo/bar/pulls/123/comments" {
					_, _ = w.Write([]byte(`[{"id":1,"commit_id":"fake-commit-id","path":"foo.txt","line":2,"body":":stop_sign: [mock](https://cloudflare.github.io/pint/checks/mock.html): syntax error\n\nsyntax details"}]`))
					return
				}
				_, _ = w.Write([]byte(""))
			}),
			summary: reporter.NewSummary([]reporter.Report{
				{
					Path: discovery.Path{
						Name:          "foo.txt",
						SymlinkTarget: "foo.txt",
					},

					ModifiedLines: []int{2},
					Rule:          mockFile.Groups[0].Rules[1],
					Problem: checks.Problem{
						Lines: diags.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "mock",
						Summary:  "syntax error",
						Details:  "syntax details",
						Severity: checks.Fatal,
					},
				},
			}),
		},
		{
			description: "maxComments 2",
			owner:       "foo",
			repo:        "bar",
			token:       "something",
			prNum:       123,
			maxComments: 2,
			timeout:     time.Second,
			httpHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet && r.URL.Path == "/api/v3/repos/foo/bar/pulls/123/reviews" {
					_, _ = w.Write([]byte(`[{"id":1,"body":"### This pull request was validated by [pint](https://github.com/cloudflare/pint).\nxxxx"}]`))
					return
				}
				if r.Method == http.MethodGet && r.URL.Path == "/api/v3/repos/foo/bar/pulls/123/comments" {
					_, _ = w.Write([]byte(`[{"id":1,"commit_id":"fake-commit-id","path":"foo.txt","line":2,"body":":stop_sign: [mock](https://cloudflare.github.io/pint/checks/mock.html): syntax error\n\nsyntax details"}]`))
					return
				}
				if r.Method == http.MethodPost && r.URL.Path == "/api/v3/repos/foo/bar/pulls/123/comments" {
					body, _ := io.ReadAll(r.Body)
					b := strings.TrimSpace(strings.TrimRight(string(body), "\n\t\r"))
					switch b {
					case `{"body":":stop_sign: **Bug** reported by [pint](https://cloudflare.github.io/pint/) **mock1** check.\n\n------\n\nsyntax error1\n\nsyntax details1\n\n------\n\n:information_source: To see documentation covering this check and instructions on how to resolve it [click here](https://cloudflare.github.io/pint/checks/mock1.html).\n","path":"foo.txt","line":2,"side":"RIGHT","commit_id":"HEAD"}`:
					case `{"body":":stop_sign: **Bug** reported by [pint](https://cloudflare.github.io/pint/) **mock2** check.\n\n------\n\nsyntax error2\n\nsyntax details2\n\n------\n\n:information_source: To see documentation covering this check and instructions on how to resolve it [click here](https://cloudflare.github.io/pint/checks/mock2.html).\n","path":"foo.txt","line":2,"side":"RIGHT","commit_id":"HEAD"}`:
					case `{"body":"This pint run would create 4 comment(s), which is more than 2 limit configured for pint.\n2 comments were skipped and won't be visibile on this PR."}`:
					default:
						t.Errorf("Unexpected comment: %s", b)
					}
				}
				_, _ = w.Write([]byte(""))
			}),
			summary: reporter.NewSummary([]reporter.Report{
				{
					Path: discovery.Path{
						Name:          "foo.txt",
						SymlinkTarget: "foo.txt",
					},

					ModifiedLines: []int{2},
					Rule:          mockFile.Groups[0].Rules[1],
					Problem: checks.Problem{
						Lines: diags.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "mock1",
						Summary:  "syntax error1",
						Details:  "syntax details1",
						Severity: checks.Bug,
					},
				},
				{
					Path: discovery.Path{
						Name:          "foo.txt",
						SymlinkTarget: "foo.txt",
					},

					ModifiedLines: []int{2},
					Rule:          mockFile.Groups[0].Rules[1],
					Problem: checks.Problem{
						Lines: diags.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "mock2",
						Summary:  "syntax error2",
						Details:  "syntax details2",
						Severity: checks.Bug,
					},
				},
				{
					Path: discovery.Path{
						Name:          "foo.txt",
						SymlinkTarget: "foo.txt",
					},

					ModifiedLines: []int{2},
					Rule:          mockFile.Groups[0].Rules[1],
					Problem: checks.Problem{
						Lines: diags.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "mock3",
						Summary:  "syntax error3",
						Details:  "syntax details3",
						Severity: checks.Fatal,
					},
				},
				{
					Path: discovery.Path{
						Name:          "foo.txt",
						SymlinkTarget: "foo.txt",
					},

					ModifiedLines: []int{2},
					Rule:          mockFile.Groups[0].Rules[1],
					Problem: checks.Problem{
						Lines: diags.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "mock4",
						Summary:  "syntax error4",
						Details:  "syntax details4",
						Severity: checks.Fatal,
					},
				},
			}),
		},
		{
			description: "maxComments 2, too many comments comment error",
			owner:       "foo",
			repo:        "bar",
			token:       "something",
			prNum:       123,
			maxComments: 2,
			timeout:     time.Second,
			httpHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet && r.URL.Path == "/api/v3/repos/foo/bar/pulls/123/reviews" {
					_, _ = w.Write([]byte(`[{"id":1,"body":"### This pull request was validated by [pint](https://github.com/cloudflare/pint).\nxxxx"}]`))
					return
				}
				if r.Method == http.MethodGet && r.URL.Path == "/api/v3/repos/foo/bar/pulls/123/comments" {
					_, _ = w.Write([]byte(`[{"id":1,"commit_id":"fake-commit-id","path":"foo.txt","line":2,"body":":stop_sign: [mock](https://cloudflare.github.io/pint/checks/mock.html): syntax error\n\nsyntax details"}]`))
					return
				}
				if r.Method == http.MethodPost && r.URL.Path == "/api/v3/repos/foo/bar/issues/123/comments" {
					body, _ := io.ReadAll(r.Body)
					b := strings.TrimSpace(strings.TrimRight(string(body), "\n\t\r"))
					if b == `{"body":"This pint run would create 4 comment(s), which is more than the limit configured for pint (2).\n2 comment(s) were skipped and won't be visibile on this PR."}` {
						w.WriteHeader(http.StatusInternalServerError)
						_, _ = w.Write([]byte("Cannot create issue comment"))
						return
					}
				}
				_, _ = w.Write([]byte(""))
			}),
			summary: reporter.NewSummary([]reporter.Report{
				{
					Path: discovery.Path{
						Name:          "foo.txt",
						SymlinkTarget: "foo.txt",
					},

					ModifiedLines: []int{2},
					Rule:          mockFile.Groups[0].Rules[1],
					Problem: checks.Problem{
						Lines: diags.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "mock1",
						Summary:  "syntax error1",
						Details:  "syntax details1",
						Severity: checks.Bug,
					},
				},
				{
					Path: discovery.Path{
						Name:          "foo.txt",
						SymlinkTarget: "foo.txt",
					},

					ModifiedLines: []int{2},
					Rule:          mockFile.Groups[0].Rules[1],
					Problem: checks.Problem{
						Lines: diags.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "mock2",
						Summary:  "syntax error2",
						Details:  "syntax details2",
						Severity: checks.Bug,
					},
				},
				{
					Path: discovery.Path{
						Name:          "foo.txt",
						SymlinkTarget: "foo.txt",
					},

					ModifiedLines: []int{2},
					Rule:          mockFile.Groups[0].Rules[1],
					Problem: checks.Problem{
						Lines: diags.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "mock3",
						Summary:  "syntax error3",
						Details:  "syntax details3",
						Severity: checks.Fatal,
					},
				},
				{
					Path: discovery.Path{
						Name:          "foo.txt",
						SymlinkTarget: "foo.txt",
					},

					ModifiedLines: []int{2},
					Rule:          mockFile.Groups[0].Rules[1],
					Problem: checks.Problem{
						Lines: diags.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "mock4",
						Summary:  "syntax error4",
						Details:  "syntax details4",
						Severity: checks.Fatal,
					},
				},
			}),
		},
		{
			description: "general comment error",
			owner:       "foo",
			repo:        "bar",
			token:       "something",
			prNum:       123,
			maxComments: 2,
			timeout:     time.Second,
			httpHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet && r.URL.Path == "/api/v3/repos/foo/bar/pulls/123/files" {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`[{"filename":"foo.txt"}]`))
					return
				}
				if r.Method == http.MethodGet && r.URL.Path == "/api/v3/repos/foo/bar/pulls/123/reviews" {
					_, _ = w.Write([]byte(`[{"id":1,"body":"### This pull request was validated by [pint](https://github.com/cloudflare/pint).\nxxxx"}]`))
					return
				}
				if r.Method == http.MethodGet && r.URL.Path == "/api/v3/repos/foo/bar/pulls/123/comments" {
					_, _ = w.Write([]byte(`[{"id":1,"commit_id":"fake-commit-id","path":"foo.txt","line":2,"body":":stop_sign: [mock](https://cloudflare.github.io/pint/checks/mock.html): syntax error\n\nsyntax details"}]`))
					return
				}
				if r.Method == http.MethodPost && r.URL.Path == "/api/v3/repos/foo/bar/issues/123/comments" {
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte("Cannot create issue comment"))
					return
				}
				_, _ = w.Write([]byte(""))
			}),
			error: func(uri string) string {
				return fmt.Sprintf("failed to create general comment: POST %s/api/v3/repos/foo/bar/issues/123/comments: 500  []", uri)
			},
			summary: reporter.NewSummary([]reporter.Report{
				{
					Path: discovery.Path{
						Name:          "foo.txt",
						SymlinkTarget: "foo.txt",
					},

					ModifiedLines: []int{2},
					Rule:          mockFile.Groups[0].Rules[1],
					Problem: checks.Problem{
						Lines: diags.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "mock1",
						Summary:  "syntax error1",
						Details:  "syntax details1",
						Severity: checks.Bug,
					},
				},
				{
					Path: discovery.Path{
						Name:          "foo.txt",
						SymlinkTarget: "foo.txt",
					},

					ModifiedLines: []int{2},
					Rule:          mockFile.Groups[0].Rules[1],
					Problem: checks.Problem{
						Lines: diags.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "mock2",
						Summary:  "syntax error2",
						Details:  "syntax details2",
						Severity: checks.Bug,
					},
				},
				{
					Path: discovery.Path{
						Name:          "foo.txt",
						SymlinkTarget: "foo.txt",
					},

					ModifiedLines: []int{2},
					Rule:          mockFile.Groups[0].Rules[1],
					Problem: checks.Problem{
						Lines: diags.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "mock3",
						Summary:  "syntax error3",
						Details:  "syntax details3",
						Severity: checks.Fatal,
					},
				},
				{
					Path: discovery.Path{
						Name:          "foo.txt",
						SymlinkTarget: "foo.txt",
					},

					ModifiedLines: []int{2},
					Rule:          mockFile.Groups[0].Rules[1],
					Problem: checks.Problem{
						Lines: diags.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "mock4",
						Summary:  "syntax error4",
						Details:  "syntax details4",
						Severity: checks.Fatal,
					},
				},
			}),
		},
		{
			description: "modified line",
			owner:       "foo",
			repo:        "bar",
			token:       "something",
			prNum:       123,
			maxComments: 50,
			timeout:     time.Second,
			httpHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet && r.URL.Path == "/api/v3/repos/foo/bar/pulls/123/reviews" {
					_, _ = w.Write([]byte(`[]`))
					return
				}
				if r.Method == http.MethodGet && r.URL.Path == "/api/v3/repos/foo/bar/pulls/123/comments" {
					_, _ = w.Write([]byte(`[]`))
					return
				}
				if r.Method == http.MethodGet && r.URL.Path == "/api/v3/repos/foo/bar/pulls/123/files" {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`[{"filename":"foo.txt", "patch": "@@ -1,4 +1,4 @@ - record: target is down\n-  expr: up == 1\n+  expr: up == 0\n - record: sum errors\n   expr: sum(errors) by (job)"}]`))
					return
				}
				if r.Method == http.MethodPost && r.URL.Path == "/api/v3/repos/foo/bar/pulls/123/comments" {
					body, _ := io.ReadAll(r.Body)
					b := strings.TrimSpace(strings.TrimRight(string(body), "\n\t\r"))
					if b != `{"body":":stop_sign: **Fatal** reported by [pint](https://cloudflare.github.io/pint/) **mock** check.\n\n------\n\nsyntax error\n\n<details>\n<summary>More information</summary>\nsyntax details\n</details>\n\n------\n\n:information_source: To see documentation covering this check and instructions on how to resolve it [click here](https://cloudflare.github.io/pint/checks/mock.html).\n","path":"foo.txt","line":1,"side":"RIGHT","commit_id":"HEAD"}` {
						t.Errorf("Unexpected comment: %s", b)
						t.FailNow()
					}
				}
				_, _ = w.Write([]byte(""))
			}),
			summary: reporter.NewSummary([]reporter.Report{
				{
					Path: discovery.Path{
						Name:          "foo.txt",
						SymlinkTarget: "foo.txt",
					},

					ModifiedLines: []int{1},
					Rule:          mockFile.Groups[0].Rules[0],
					Problem: checks.Problem{
						Lines: diags.LineRange{
							First: 1,
							Last:  1,
						},
						Reporter: "mock",
						Summary:  "syntax error",
						Details:  "syntax details",
						Severity: checks.Fatal,
					},
				},
			}),
		},
		{
			description: "unmodified line",
			owner:       "foo",
			repo:        "bar",
			token:       "something",
			prNum:       123,
			maxComments: 50,
			timeout:     time.Second,
			httpHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet && r.URL.Path == "/api/v3/repos/foo/bar/pulls/123/reviews" {
					_, _ = w.Write([]byte(`[]`))
					return
				}
				if r.Method == http.MethodGet && r.URL.Path == "/api/v3/repos/foo/bar/pulls/123/comments" {
					_, _ = w.Write([]byte(`[]`))
					return
				}
				if r.Method == http.MethodGet && r.URL.Path == "/api/v3/repos/foo/bar/pulls/123/files" {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`[{"filename":"foo.txt", "patch": "@@ -1,4 +1,4 @@ - record: target is down\n-  expr: up == 1\n+  expr: up == 0\n - record: sum errors\n   expr: sum(errors) by (job)"}]`))
					return
				}
				if r.Method == http.MethodPost && r.URL.Path == "/api/v3/repos/foo/bar/pulls/123/comments" {
					body, _ := io.ReadAll(r.Body)
					b := strings.TrimSpace(strings.TrimRight(string(body), "\n\t\r"))
					if b != `{"body":":stop_sign: **Fatal** reported by [pint](https://cloudflare.github.io/pint/) **mock** check.\n\n------\n\nsyntax error\n\n<details>\n<summary>More information</summary>\nsyntax details\n</details>\n\n------\n\n:information_source: To see documentation covering this check and instructions on how to resolve it [click here](https://cloudflare.github.io/pint/checks/mock.html).\n","path":"foo.txt","line":1,"side":"RIGHT","commit_id":"HEAD"}` {
						t.Errorf("Unexpected comment: %s", b)
						t.FailNow()
					}
				}
				_, _ = w.Write([]byte(""))
			}),
			summary: reporter.NewSummary([]reporter.Report{
				{
					Path: discovery.Path{
						Name:          "foo.txt",
						SymlinkTarget: "foo.txt",
					},

					ModifiedLines: []int{2},
					Rule:          mockFile.Groups[0].Rules[1],
					Problem: checks.Problem{
						Lines: diags.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "mock",
						Summary:  "syntax error",
						Details:  "syntax details",
						Severity: checks.Fatal,
					},
				},
			}),
		},
		{
			description: "removed line",
			owner:       "foo",
			repo:        "bar",
			token:       "something",
			prNum:       123,
			maxComments: 50,
			timeout:     time.Second,
			httpHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet && r.URL.Path == "/api/v3/repos/foo/bar/pulls/123/reviews" {
					_, _ = w.Write([]byte(`[]`))
					return
				}
				if r.Method == http.MethodGet && r.URL.Path == "/api/v3/repos/foo/bar/pulls/123/comments" {
					_, _ = w.Write([]byte(`[]`))
					return
				}
				if r.Method == http.MethodGet && r.URL.Path == "/api/v3/repos/foo/bar/pulls/123/files" {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`[{"filename":"foo.txt", "patch": "@@ -1,5 +1,4 @@\n - record: target is down\n   expr: up == 0\n-  labels: {}\n - record: sum errors\n   expr: sum(errors) by (job)"}]`))
					return
				}
				if r.Method == http.MethodPost && r.URL.Path == "/api/v3/repos/foo/bar/pulls/123/comments" {
					body, _ := io.ReadAll(r.Body)
					b := strings.TrimSpace(strings.TrimRight(string(body), "\n\t\r"))
					if b != `{"body":":stop_sign: **Fatal** reported by [pint](https://cloudflare.github.io/pint/) **mock** check.\n\n------\n\nsyntax error\n\n<details>\n<summary>More information</summary>\nsyntax details\n</details>\n\n------\n\n:information_source: To see documentation covering this check and instructions on how to resolve it [click here](https://cloudflare.github.io/pint/checks/mock.html).\n","path":"foo.txt","line":3,"side":"LEFT","commit_id":"HEAD"}` {
						t.Errorf("Unexpected comment: %s", b)
						t.FailNow()
					}
				}
				_, _ = w.Write([]byte(""))
			}),
			summary: reporter.NewSummary([]reporter.Report{
				{
					Path: discovery.Path{
						Name:          "foo.txt",
						SymlinkTarget: "foo.txt",
					},

					ModifiedLines: []int{3},
					Rule:          mockFile.Groups[0].Rules[0],
					Problem: checks.Problem{
						Lines: diags.LineRange{
							First: 3,
							Last:  3,
						},
						Reporter: "mock",
						Summary:  "syntax error",
						Details:  "syntax details",
						Severity: checks.Fatal,
						Anchor:   checks.AnchorBefore,
					},
				},
			}),
		},
		{
			description: "review comment",
			owner:       "foo",
			repo:        "bar",
			token:       "something",
			prNum:       123,
			maxComments: 50,
			timeout:     time.Second,
			httpHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost && r.URL.Path == "/api/v3/repos/foo/bar/pulls/123/reviews" {
					body, _ := io.ReadAll(r.Body)
					b := strings.TrimSpace(strings.TrimRight(string(body), "\n\t\r"))
					if b != `{"commit_id":"HEAD","body":"### This pull request was validated by [pint](https://github.com/cloudflare/pint).\n:heavy_exclamation_mark:\tProblems found.\n| Severity | Number of problems |\n| --- | --- |\n| Fatal | 1 |\n<details><summary>Stats</summary>\n<p>\n\n| Stat | Value |\n| --- | --- |\n| Version | v0.0.0 |\n| Number of rules parsed | 0 |\n| Number of rules checked | 0 |\n| Number of problems found | 1 |\n| Number of offline checks | 0 |\n| Number of online checks | 0 |\n| Checks duration | 0 |\n\n</p>\n</details>\n\n<details><summary>Problems</summary>\n<p>\n\nFailed to generate list of problems: open foo.txt: no such file or directory\n</p>\n</details>\n\n","event":"COMMENT"}` {
						t.Errorf("Unexpected comment: %s", b)
						t.FailNow()
					}
				}
				_, _ = w.Write([]byte(""))
			}),
			summary: reporter.NewSummary([]reporter.Report{
				{
					Path: discovery.Path{
						Name:          "foo.txt",
						SymlinkTarget: "foo.txt",
					},

					ModifiedLines: []int{1},
					Rule:          mockFile.Groups[0].Rules[0],
					Problem: checks.Problem{
						Lines: diags.LineRange{
							First: 1,
							Last:  1,
						},
						Reporter: "mock",
						Summary:  "syntax error",
						Details:  "syntax details",
						Severity: checks.Fatal,
					},
				},
			}),
		},
		{
			description: "prometheus details",
			owner:       "foo",
			repo:        "bar",
			token:       "something",
			prNum:       123,
			maxComments: 50,
			timeout:     time.Second,
			httpHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost && r.URL.Path == "/api/v3/repos/foo/bar/pulls/123/reviews" {
					expected := `### This pull request was validated by [pint](https://github.com/cloudflare/pint).
:heavy_check_mark: No problems found
<details><summary>Stats</summary>
<p>

| Stat | Value |
| --- | --- |
| Version | v0.0.0 |
| Number of rules parsed | 0 |
| Number of rules checked | 0 |
| Number of problems found | 0 |
| Number of offline checks | 0 |
| Number of online checks | 0 |
| Checks duration | 0 |

</p>
</details>

<details><summary>Problems</summary>
<p>

No problems reported
</p>
</details>

Some checks were disabled because one or more configured Prometheus server doesn't seem to support all required Prometheus APIs.
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
`
					body, _ := io.ReadAll(r.Body)
					type jr struct {
						Body string
					}
					var r jr
					_ = json.Unmarshal(body, &r)
					if diff := cmp.Diff(expected, r.Body); diff != "" {
						t.Error(diff)
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					_, _ = w.Write([]byte(`{}`))
				}
			}),
			summary: summaryWithDetails,
		},
	} {
		t.Run(tc.description, func(t *testing.T) {
			slog.SetDefault(slogt.New(t))

			var handler http.Handler
			if tc.httpHandler != nil {
				handler = tc.httpHandler
			} else {
				// Handler that checks for token.
				handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					auth := r.Header["Authorization"]
					if len(auth) == 0 {
						w.WriteHeader(http.StatusInternalServerError)
						_, _ = w.Write([]byte("No token"))
						t.Fatal("got a request with no token")
						return
					}
					token := auth[0]
					if token != "Bearer "+tc.token {
						w.WriteHeader(http.StatusInternalServerError)
						_, _ = w.Write([]byte("Invalid token"))
						t.Fatalf("got a request with invalid token (got %s)", token)
					}
				})
			}
			srv := httptest.NewServer(handler)
			defer srv.Close()
			r, err := reporter.NewGithubReporter(
				"v0.0.0",
				srv.URL,
				srv.URL,
				tc.timeout,
				tc.token,
				tc.owner,
				tc.repo,
				tc.prNum,
				tc.maxComments,
				"HEAD",
			)
			require.NoError(t, err)

			err = reporter.Submit(t.Context(), tc.summary, r)
			if tc.error == nil {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.error(srv.URL))
			}
		})
	}
}
