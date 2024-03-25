package reporter_test

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/neilotoole/slogt"
	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/git"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/reporter"
)

func TestGithubReporter(t *testing.T) {
	type testCaseT struct {
		description string
		reports     []reporter.Report
		httpHandler http.Handler
		error       func(uri string) string
		gitCmd      git.CommandRunner

		owner       string
		repo        string
		token       string
		prNum       int
		maxComments int
		timeout     time.Duration
	}

	p := parser.NewParser()
	mockRules, _ := p.Parse([]byte(`
- record: target is down
  expr: up == 0
- record: sum errors
  expr: sum(errors) by (job)
`))

	blameLine := func(sha string, line int, filename, content string) string {
		return fmt.Sprintf(`%s %d %d 1
filename %s
	%s
`, sha, line, line, filename, content)
	}

	for _, tc := range []testCaseT{
		{
			description: "timeout errors out",
			owner:       "foo",
			repo:        "bar",
			token:       "something",
			prNum:       123,
			maxComments: 50,
			httpHandler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				time.Sleep(1 * time.Second)
				_, _ = w.Write([]byte("OK"))
			}),
			timeout: 100 * time.Millisecond,
			gitCmd: func(args ...string) ([]byte, error) {
				if args[0] == "rev-parse" {
					return []byte("fake-commit-id"), nil
				}
				if args[0] == "blame" {
					content := blameLine("fake-commit-id", 2, "foo.txt", "up == 0")
					return []byte(content), nil
				}
				return nil, nil
			},
			error: func(_ string) string {
				return "failed to list pull request reviews: context deadline exceeded"
			},
			reports: []reporter.Report{
				{
					SourcePath:    "foo.txt",
					ModifiedLines: []int{2},
					Rule:          mockRules[1],
					Problem: checks.Problem{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "mock",
						Text:     "syntax error",
						Severity: checks.Fatal,
					},
				},
			},
		},
		{
			description: "list reviews error",
			owner:       "foo",
			repo:        "bar",
			token:       "something",
			prNum:       123,
			maxComments: 50,
			timeout:     time.Second,
			gitCmd: func(args ...string) ([]byte, error) {
				if args[0] == "rev-parse" {
					return []byte("fake-commit-id"), nil
				}
				if args[0] == "blame" {
					content := blameLine("fake-commit-id", 2, "foo.txt", "up == 0")
					return []byte(content), nil
				}
				return nil, nil
			},
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
			reports: []reporter.Report{
				{
					SourcePath:    "foo.txt",
					ModifiedLines: []int{2},
					Rule:          mockRules[1],
					Problem: checks.Problem{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "mock",
						Text:     "syntax error",
						Details:  "syntax details",
						Severity: checks.Fatal,
					},
				},
			},
		},
		{
			description: "happy path",
			owner:       "foo",
			repo:        "bar",
			token:       "something",
			prNum:       123,
			maxComments: 50,
			timeout:     time.Second,
			gitCmd: func(args ...string) ([]byte, error) {
				if args[0] == "rev-parse" {
					return []byte("fake-commit-id"), nil
				}
				if args[0] == "blame" {
					content := blameLine("fake-commit-id", 2, "foo.txt", "up == 0")
					return []byte(content), nil
				}
				return nil, nil
			},
			reports: []reporter.Report{
				{
					SourcePath:    "foo.txt",
					ModifiedLines: []int{2},
					Rule:          mockRules[1],
					Problem: checks.Problem{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "mock",
						Text:     "syntax error",
						Details:  "syntax details",
						Severity: checks.Fatal,
					},
				},
			},
		},
		{
			description: "error crating review",
			owner:       "foo",
			repo:        "bar",
			token:       "something",
			prNum:       123,
			maxComments: 50,
			timeout:     time.Second,
			gitCmd: func(args ...string) ([]byte, error) {
				if args[0] == "rev-parse" {
					return []byte("fake-commit-id"), nil
				}
				if args[0] == "blame" {
					content := blameLine("fake-commit-id", 2, "foo.txt", "up == 0")
					return []byte(content), nil
				}
				return nil, nil
			},
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
			reports: []reporter.Report{
				{
					SourcePath:    "foo.txt",
					ModifiedLines: []int{2},
					Rule:          mockRules[1],
					Problem: checks.Problem{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "mock",
						Text:     "syntax error",
						Details:  "syntax details",
						Severity: checks.Fatal,
					},
				},
			},
		},
		{
			description: "error updating existing review",
			owner:       "foo",
			repo:        "bar",
			token:       "something",
			prNum:       123,
			maxComments: 50,
			timeout:     time.Second,
			gitCmd: func(args ...string) ([]byte, error) {
				if args[0] == "rev-parse" {
					return []byte("fake-commit-id"), nil
				}
				if args[0] == "blame" {
					content := blameLine("fake-commit-id", 2, "foo.txt", "up == 0")
					return []byte(content), nil
				}
				return nil, nil
			},
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
			reports: []reporter.Report{
				{
					SourcePath:    "foo.txt",
					ModifiedLines: []int{2},
					Rule:          mockRules[1],
					Problem: checks.Problem{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "mock",
						Text:     "syntax error",
						Details:  "syntax details",
						Severity: checks.Fatal,
					},
				},
			},
		},
		{
			description: "update existing review",
			owner:       "foo",
			repo:        "bar",
			token:       "something",
			prNum:       123,
			maxComments: 50,
			timeout:     time.Second,
			gitCmd: func(args ...string) ([]byte, error) {
				if args[0] == "rev-parse" {
					return []byte("fake-commit-id"), nil
				}
				if args[0] == "blame" {
					content := blameLine("fake-commit-id", 2, "foo.txt", "up == 0")
					return []byte(content), nil
				}
				return nil, nil
			},
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
			reports: []reporter.Report{
				{
					SourcePath:    "foo.txt",
					ModifiedLines: []int{2},
					Rule:          mockRules[1],
					Problem: checks.Problem{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "mock",
						Text:     "syntax error",
						Details:  "syntax details",
						Severity: checks.Fatal,
					},
				},
			},
		},
		{
			description: "maxComments 2",
			owner:       "foo",
			repo:        "bar",
			token:       "something",
			prNum:       123,
			maxComments: 2,
			timeout:     time.Second,
			gitCmd: func(args ...string) ([]byte, error) {
				if args[0] == "rev-parse" {
					return []byte("fake-commit-id"), nil
				}
				if args[0] == "blame" {
					content := blameLine("fake-commit-id", 2, "foo.txt", "up == 0")
					return []byte(content), nil
				}
				return nil, nil
			},
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
					case `{"body":":stop_sign: [mock1](https://cloudflare.github.io/pint/checks/mock1.html): syntax error1\n\nsyntax details1","path":"","line":2,"side":"RIGHT","commit_id":"fake-commit-id"}`:
					case `{"body":":stop_sign: [mock2](https://cloudflare.github.io/pint/checks/mock2.html): syntax error2\n\nsyntax details2","path":"","line":2,"side":"RIGHT","commit_id":"fake-commit-id"}`:
					case `{"body":"This pint run would create 4 comment(s), which is more than 2 limit configured for pint.\n2 comments were skipped and won't be visibile on this PR.","commit_id":"fake-commit-id"}`:
					default:
						t.Errorf("Unexpected comment: %s", b)
					}
				}
				_, _ = w.Write([]byte(""))
			}),
			reports: []reporter.Report{
				{
					SourcePath:    "foo.txt",
					ModifiedLines: []int{2},
					Rule:          mockRules[1],
					Problem: checks.Problem{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "mock1",
						Text:     "syntax error1",
						Details:  "syntax details1",
						Severity: checks.Bug,
					},
				},
				{
					SourcePath:    "foo.txt",
					ModifiedLines: []int{2},
					Rule:          mockRules[1],
					Problem: checks.Problem{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "mock2",
						Text:     "syntax error2",
						Details:  "syntax details2",
						Severity: checks.Bug,
					},
				},
				{
					SourcePath:    "foo.txt",
					ModifiedLines: []int{2},
					Rule:          mockRules[1],
					Problem: checks.Problem{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "mock3",
						Text:     "syntax error3",
						Details:  "syntax details3",
						Severity: checks.Fatal,
					},
				},
				{
					SourcePath:    "foo.txt",
					ModifiedLines: []int{2},
					Rule:          mockRules[1],
					Problem: checks.Problem{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "mock4",
						Text:     "syntax error4",
						Details:  "syntax details4",
						Severity: checks.Fatal,
					},
				},
			},
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
						w.WriteHeader(500)
						_, _ = w.Write([]byte("No token"))
						t.Fatal("got a request with no token")
						return
					}
					token := auth[0]
					if token != fmt.Sprintf("Bearer %s", tc.token) {
						w.WriteHeader(500)
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
				tc.gitCmd,
			)
			require.NoError(t, err)

			err = r.Submit(reporter.NewSummary(tc.reports))
			if tc.error == nil {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.error(srv.URL))
			}
		})
	}
}
