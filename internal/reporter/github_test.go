package reporter_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/git"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/reporter"
)

func TestGithubReporter(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.FatalLevel)

	type testCaseT struct {
		description string
		reports     []reporter.Report
		httpHandler http.Handler
		error       string
		gitCmd      git.CommandRunner

		owner   string
		repo    string
		token   string
		prNum   int
		timeout time.Duration
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
			httpHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
			error: "failed to list pull request reviews: context deadline exceeded",
			reports: []reporter.Report{
				{
					SourcePath:    "foo.txt",
					ModifiedLines: []int{2},
					Rule:          mockRules[1],
					Problem: checks.Problem{
						Fragment: "syntax error",
						Lines:    []int{2},
						Reporter: "mock",
						Text:     "syntax error",
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
			timeout:     1000 * time.Millisecond,
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
						Fragment: "syntax error",
						Lines:    []int{2},
						Reporter: "mock",
						Text:     "syntax error",
						Severity: checks.Fatal,
					},
				},
			},
		},
	} {
		t.Run(tc.description, func(t *testing.T) {
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
				"v0.999",
				srv.URL,
				srv.URL,
				tc.timeout,
				tc.token,
				tc.owner,
				tc.repo,
				tc.prNum,
				tc.gitCmd,
			)
			require.NoError(t, err)

			err = r.Submit(reporter.NewSummary(tc.reports))
			if tc.error == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.error)
			}
		})
	}
}
