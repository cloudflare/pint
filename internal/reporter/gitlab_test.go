package reporter_test

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/neilotoole/slogt"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/reporter"
)

func TestGitLabReporter(t *testing.T) {
	type errorCheck func(err error) error

	type testCaseT struct {
		httpHandler  http.Handler
		errorHandler errorCheck

		description string
		branch      string
		token       string

		reports     []reporter.Report
		timeout     time.Duration
		project     int
		maxComments int
	}

	p := parser.NewParser()
	mockRules, _ := p.Parse([]byte(`
- record: target is down
  expr: up == 0
- record: sum errors
  expr: sum(errors) by (job)
`))

	testCases := []testCaseT{
		{
			description: "returns an error on non-200 HTTP response",
			branch:      "fakeBranch",
			token:       "fakeToken",
			timeout:     time.Second,
			project:     123,
			maxComments: 0,
			reports: []reporter.Report{
				{
					Path: discovery.Path{
						SymlinkTarget: "foo.txt",
						Name:          "foo.txt",
					},
					ModifiedLines: []int{2},
					Rule:          mockRules[0],
					Problem:       checks.Problem{},
				},
			},
			httpHandler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(400)
				_, _ = w.Write([]byte("Bad Request"))
			}),
			errorHandler: func(err error) error {
				if err != nil && strings.HasPrefix(err.Error(), "failed to get GitLab user ID:") {
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
			maxComments: 0,
			reports: []reporter.Report{
				{
					Path: discovery.Path{
						SymlinkTarget: "foo.txt",
						Name:          "foo.txt",
					},
					ModifiedLines: []int{2},
					Rule:          mockRules[0],
					Problem:       checks.Problem{},
				},
			},
			httpHandler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				time.Sleep(time.Second * 2)
				w.WriteHeader(400)
				_, _ = w.Write([]byte("Bad Request"))
			}),
			errorHandler: func(err error) error {
				if err != nil && strings.HasSuffix(err.Error(), "context deadline exceeded") {
					return nil
				}
				return fmt.Errorf("wrong error: %w", err)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			slog.SetDefault(slogt.New(t))

			srv := httptest.NewServer(tc.httpHandler)
			defer srv.Close()

			r, err := reporter.NewGitLabReporter(
				"v0.0.0",
				tc.branch,
				srv.URL,
				tc.timeout,
				tc.token,
				tc.project,
				tc.maxComments,
			)
			if err == nil {
				summary := reporter.NewSummary(tc.reports)
				err = reporter.Submit(context.Background(), summary, r)
			}
			if e := tc.errorHandler(err); e != nil {
				t.Errorf("error check failure: %s", e)
				return
			}
		})
	}
}
