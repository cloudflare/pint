package reporter_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/neilotoole/slogt"
	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/reporter"
)

func TestGitLabReporterBadBaseURI(t *testing.T) {
	_, err := reporter.NewGitLabReporter(
		"v0.0.0",
		"branch",
		"%gh&%ij",
		time.Second,
		"token",
		123,
		0,
	)
	require.Error(t, err)
}

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

	p := parser.NewParser(false)
	mockRules, _ := p.Parse([]byte(`
- record: target is down
  expr: up == 0
- record: sum errors
  expr: sum(errors) by (job)
`))

	fooReport := reporter.Report{
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
	fooDiff := `@@ -1,4 +1,6 @@\n- record: target is down\n-  expr: up == 0\n+  expr: up == 1\n+  labels:\n+    foo: bar\n- record: sum errors\nexpr: sum(errors) by (job)\n`

	testCases := []testCaseT{
		{
			description: "empty list of merge requests",
			branch:      "fakeBranch",
			token:       "fakeToken",
			timeout:     time.Second,
			project:     123,
			maxComments: 50,
			reports:     []reporter.Report{fooReport},
			httpHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(200)
				switch r.URL.Path {
				case "/api/v4/user":
					_, _ = w.Write([]byte(`{"id": 123}`))
				default:
					_, _ = w.Write([]byte("[]"))
				}
			}),
			errorHandler: func(err error) error {
				return err
			},
		},
		{
			description: "multiple merge requests",
			branch:      "fakeBranch",
			token:       "fakeToken",
			timeout:     time.Second,
			project:     123,
			maxComments: 50,
			reports:     []reporter.Report{fooReport},
			httpHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(200)
				switch r.URL.Path {
				case "/api/v4/user":
					if r.Method == http.MethodGet {
						_, _ = w.Write([]byte(`{"id": 123}`))
					}
				case "/api/v4/projects/123/merge_requests":
					if r.Method == http.MethodGet {
						_, _ = w.Write([]byte(`[{"iid":1},{"iid":2},{"iid":5}]`))
					}
				case "/api/v4/projects/123/merge_requests/1/diffs", "/api/v4/projects/123/merge_requests/2/diffs", "/api/v4/projects/123/merge_requests/5/diffs":
					if r.Method == http.MethodGet {
						_, _ = w.Write([]byte(`[{"diff":"` + fooDiff + `","new_path":"foo.txt","old_path":"foo.txt"}]`))
					}
				case "/api/v4/projects/123/merge_requests/1/versions", "/api/v4/projects/123/merge_requests/2/versions", "/api/v4/projects/123/merge_requests/5/versions":
					if r.Method == http.MethodGet {
						_, _ = w.Write([]byte(`[
{"id": 2,"head_commit_sha": "head","base_commit_sha": "base","start_commit_sha": "start"},
{"id": 1,"head_commit_sha": "head","base_commit_sha": "base","start_commit_sha": "start"}
]`))
					}
				case "/api/v4/projects/123/merge_requests/1/discussions", "/api/v4/projects/123/merge_requests/2/discussions":
					if r.Method == http.MethodGet {
						_, _ = w.Write([]byte(`[
{"id":"100","notes":[
	{"id":101,
	 "system":true,
	 "author":{"id":123},
	 "body":"system message"
	}
]},
{"id":"200","system":false,"notes":[
	{"id":201,
	"system":false,
	"author":{"id":321},
	"position":{"base_sha": "base","start_sha": "start","head_sha": "head","old_path": "foo.txt","new_path": "foo.txt","position_type": "text","new_line": 2},
	"body":"different user"
   }
]},
{"id":"300","system":false,"notes":[
	{"id":301,
	"system":false,
	"author":{"id":123},
	"position":{"base_sha": "base","start_sha": "start","head_sha": "head","old_path": "foo.txt","new_path": "foo.txt","position_type": "text","new_line": 2},
	"body":"stale comment"
   }
]},
{"id":"400","system":false,"notes":[
	{"id":401,
	"system":false,
	"author":{"id":123},
	"position":{"base_sha": "base","start_sha": "start","head_sha": "head","old_path": "foo.txt","new_path": "foo.txt","position_type": "text","new_line": 1},
	"body":"stale comment on unmodified line"
   }
]}
]`))
					} else {
						_, _ = w.Write([]byte(`{}`))
					}
				case "/api/v4/projects/123/merge_requests/5/discussions":
					if r.Method == http.MethodGet {
						_, _ = w.Write([]byte(`[]`))
					} else {
						_, _ = w.Write([]byte(`{}`))
					}
				}
			}),
			errorHandler: func(err error) error {
				return err
			},
		},
		{
			description: "list merge requests failed",
			branch:      "fakeBranch",
			token:       "fakeToken",
			timeout:     time.Minute,
			project:     123,
			maxComments: 50,
			reports:     []reporter.Report{fooReport},
			httpHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/v4/user":
					if r.Method == http.MethodGet {
						_, _ = w.Write([]byte(`{"id": 123}`))
					}
				case "/api/v4/projects/123/merge_requests":
					w.WriteHeader(500)
					_, _ = w.Write([]byte("Mock error"))
				default:
					w.WriteHeader(200)
					if r.Method == http.MethodGet {
						_, _ = w.Write([]byte(`[]`))
					}
				}
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
			branch:      "fakeBranch",
			token:       "fakeToken",
			timeout:     time.Minute,
			project:     123,
			maxComments: 50,
			reports:     []reporter.Report{fooReport},
			httpHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/v4/user":
					w.WriteHeader(500)
					_, _ = w.Write([]byte("Mock error"))
				default:
					w.WriteHeader(200)
					if r.Method == http.MethodGet {
						_, _ = w.Write([]byte(`[]`))
					}
				}
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
			branch:      "fakeBranch",
			token:       "fakeToken",
			timeout:     time.Second,
			project:     123,
			maxComments: 1,
			reports: []reporter.Report{
				{
					Path: discovery.Path{
						SymlinkTarget: "foo.txt",
						Name:          "foo.txt",
					},
					ModifiedLines: []int{1},
					Rule:          mockRules[0],
					Problem: checks.Problem{
						Reporter: "foo",
						Text:     "foo error",
						Details:  "foo details",
						Lines:    parser.LineRange{First: 1, Last: 3},
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
					Rule:          mockRules[0],
					Problem: checks.Problem{
						Reporter: "foo",
						Text:     "foo error",
						Details:  "foo details",
						Lines:    parser.LineRange{First: 1, Last: 3},
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
					Rule:          mockRules[0],
					Problem: checks.Problem{
						Reporter: "foo",
						Text:     "foo error",
						Details:  "foo details",
						Lines:    parser.LineRange{First: 1, Last: 3},
						Severity: checks.Fatal,
						Anchor:   checks.AnchorAfter,
					},
				},
			},
			httpHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(200)
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
						_, _ = w.Write([]byte(`[{"diff":"` + fooDiff + `","new_path":"foo.txt","old_path":"foo.txt"}]`))
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
						_, _ = w.Write([]byte(`{}`))
					}
				}
			}),
			errorHandler: func(err error) error {
				return err
			},
		},
		{
			description: "no diff",
			branch:      "fakeBranch",
			token:       "fakeToken",
			timeout:     time.Second,
			project:     123,
			maxComments: 1,
			reports:     []reporter.Report{fooReport},
			httpHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(200)
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
						_, _ = w.Write([]byte(`[]`))
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

						_, _ = w.Write([]byte(`ERROR`))
						t.FailNow()
					}
				}
			}),
			errorHandler: func(err error) error {
				return err
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
				require.NoError(t, tc.errorHandler(err))
			}
			require.NoError(t, tc.errorHandler(err))
		})
	}
}
