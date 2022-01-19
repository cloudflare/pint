package reporter_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/rs/zerolog"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/git"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/reporter"
)

func blameLine(sha string, line int, filename, content string) string {
	return fmt.Sprintf(`%s %d %d 1
filename %s
	%s
`, sha, line, line, filename, content)
}

func TestBitBucketReporter(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.FatalLevel)

	type errorCheck func(err error) error

	type testCaseT struct {
		description  string
		gitCmd       git.CommandRunner
		summary      reporter.Summary
		httpHandler  http.Handler
		report       reporter.BitBucketReport
		annotations  reporter.BitBucketAnnotations
		errorHandler errorCheck
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
			description: "returns an error on git head failure",
			gitCmd: func(args ...string) ([]byte, error) {
				if args[0] == "rev-parse" {
					return nil, errors.New("git head error")
				}
				return nil, nil
			},
			errorHandler: func(err error) error {
				if err != nil && err.Error() == "failed to get HEAD commit: git head error" {
					return nil
				}
				return fmt.Errorf("Expected git head error, got %w", err)
			},
		},
		{
			description: "returns an error on git blame failure",
			gitCmd: func(args ...string) ([]byte, error) {
				if args[0] == "rev-parse" {
					return []byte("fake-commit-id"), nil
				}
				if args[0] == "blame" {
					return nil, errors.New("git blame error")
				}
				return nil, nil
			},
			summary: reporter.Summary{
				Reports: []reporter.Report{{Path: "foo.txt", Rule: mockRules[0], Problem: checks.Problem{}}},
			},
			errorHandler: func(err error) error {
				if err != nil && err.Error() == "failed to run git blame: git blame error" {
					return nil
				}
				return fmt.Errorf("Expected git head error, got %w", err)
			},
		},
		{
			description: "returns an error on non-200 HTTP response",
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
			summary: reporter.Summary{
				Reports: []reporter.Report{{Path: "foo.txt", Rule: mockRules[0], Problem: checks.Problem{}}},
			},
			httpHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(400)
				_, _ = w.Write([]byte("Bad Request"))
			}),
			errorHandler: func(err error) error {
				if err != nil && err.Error() == "failed to create BitBucket report: PUT request failed" {
					return nil
				}
				return fmt.Errorf("Expected 'PUT request failed', got %w", err)
			},
		},
		{
			description: "returns an error on HTTP response headers timeout",
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
			summary: reporter.Summary{
				Reports: []reporter.Report{{Path: "foo.txt", Rule: mockRules[0], Problem: checks.Problem{}}},
			},
			httpHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(time.Second * 2)
				w.WriteHeader(400)
				_, _ = w.Write([]byte("Bad Request"))
			}),
			errorHandler: func(err error) error {
				var neterr net.Error
				if ok := errors.As(errors.Unwrap(err), &neterr); ok && neterr.Timeout() {
					return nil
				}
				return fmt.Errorf("Expected a timeout error, got %w", err)
			},
		},
		{
			description: "returns an error on HTTP response body timeout",
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
			summary: reporter.Summary{
				Reports: []reporter.Report{{Path: "foo.txt", Rule: mockRules[0], Problem: checks.Problem{}}},
			},
			httpHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(400)
				time.Sleep(time.Second * 2)
				_, _ = w.Write([]byte("Bad Request"))
			}),
			errorHandler: func(err error) error {
				var neterr net.Error
				if ok := errors.As(errors.Unwrap(err), &neterr); ok && neterr.Timeout() {
					return nil
				}
				return fmt.Errorf("Expected a timeout error, got %w", err)
			},
		},
		{
			description: "sends a correct report",
			gitCmd: func(args ...string) ([]byte, error) {
				if args[0] == "rev-parse" {
					return []byte("fake-commit-id"), nil
				}
				if args[0] == "blame" {
					content := blameLine("fake-commit-id", 2, "foo.txt", "up == 0") + blameLine("fake-commit-id", 4, "foo.txt", "errors")
					return []byte(content), nil
				}
				return nil, nil
			},
			summary: reporter.Summary{
				Reports: []reporter.Report{
					{
						Path: "foo.txt",
						Rule: mockRules[1],
						Problem: checks.Problem{
							Fragment: "up",
							Lines:    []int{1},
							Reporter: "mock",
							Text:     "this should be ignored, line is not part of the diff",
							Severity: checks.Bug,
						},
					},
					{
						Path: "bar.txt",
						Rule: mockRules[1],
						Problem: checks.Problem{
							Fragment: "up",
							Lines:    []int{1},
							Reporter: "mock",
							Text:     "this should be ignored, file is not part of the diff",
							Severity: checks.Bug,
						},
					},
					{
						Path: "foo.txt",
						Rule: mockRules[1],
						Problem: checks.Problem{
							Fragment: "up",
							Lines:    []int{2},
							Reporter: "mock",
							Text:     "bad name",
							Severity: checks.Fatal,
						},
					},
					{
						Path: "foo.txt",
						Rule: mockRules[0],
						Problem: checks.Problem{
							Fragment: "up == 0",
							Lines:    []int{2},
							Reporter: "mock",
							Text:     "mock text",
							Severity: checks.Bug,
						},
					},
					{
						Path: "foo.txt",
						Rule: mockRules[1],
						Problem: checks.Problem{
							Fragment: "errors",
							Lines:    []int{4},
							Reporter: "mock",
							Text:     "mock text 2",
							Severity: checks.Warning,
						},
					},
				},
				FileChanges: discovery.NewFileCommitsFromMap(map[string][]string{"foo.txt": {"fake-commit-id"}}),
			},
			report: reporter.BitBucketReport{
				Title:  "Pint - Prometheus rules linter",
				Result: "FAIL",
			},
			annotations: reporter.BitBucketAnnotations{
				Annotations: []reporter.BitBucketAnnotation{
					{
						Path:     "foo.txt",
						Line:     2,
						Message:  "mock: bad name",
						Severity: "HIGH",
						Type:     "BUG",
					},
					{
						Path:     "foo.txt",
						Line:     2,
						Message:  "mock: mock text",
						Severity: "MEDIUM",
						Type:     "BUG",
					},
					{
						Path:     "foo.txt",
						Line:     4,
						Message:  "mock: mock text 2",
						Severity: "LOW",
						Type:     "CODE_SMELL",
					},
				},
			},
			errorHandler: func(err error) error {
				if err.Error() != "fatal error(s) reported" {
					return fmt.Errorf("Unpexpected error: %w", err)
				}
				return nil
			},
		},
		{
			description: "FATAL errors are always reported, regardless of line number",
			gitCmd: func(args ...string) ([]byte, error) {
				if args[0] == "rev-parse" {
					return []byte("fake-commit-id"), nil
				}
				if args[0] == "blame" {
					content :=
						blameLine("fake-commit-00", 1, "foo.txt", "ignore") +
							blameLine("fake-commit-00", 2, "foo.txt", "ignore") +
							blameLine("fake-commit-id", 3, "foo.txt", "ok") +
							blameLine("fake-commit-id", 4, "foo.txt", "syntax error") +
							blameLine("fake-commit-01", 5, "foo.txt", "ignore")
					return []byte(content), nil
				}
				return nil, nil
			},
			summary: reporter.Summary{
				Reports: []reporter.Report{
					{
						Path: "foo.txt",
						Rule: mockRules[1],
						Problem: checks.Problem{
							Fragment: "syntax error",
							Lines:    []int{1},
							Reporter: "mock",
							Text:     "syntax error",
							Severity: checks.Fatal,
						},
					},
				},
				FileChanges: discovery.NewFileCommitsFromMap(map[string][]string{"foo.txt": {"fake-commit-id"}}),
			},
			report: reporter.BitBucketReport{
				Title:  "Pint - Prometheus rules linter",
				Result: "FAIL",
			},
			annotations: reporter.BitBucketAnnotations{
				Annotations: []reporter.BitBucketAnnotation{
					{
						Path:     "foo.txt",
						Line:     3,
						Message:  "mock: syntax error",
						Severity: "HIGH",
						Type:     "BUG",
					},
				},
			},
			errorHandler: func(err error) error {
				if err.Error() != "fatal error(s) reported" {
					return fmt.Errorf("Unpexpected error: %w", err)
				}
				return nil
			},
		},
		{
			description: "sends a correct empty report",
			gitCmd: func(args ...string) ([]byte, error) {
				if args[0] == "rev-parse" {
					return []byte("fake-commit-id"), nil
				}
				if args[0] == "blame" {
					content := blameLine("fake-commit-id", 2, "foo.txt", "up == 0") + blameLine("fake-commit-id", 4, "foo.txt", "errors")
					return []byte(content), nil
				}
				return nil, nil
			},
			summary: reporter.Summary{},
			report: reporter.BitBucketReport{
				Title:  "Pint - Prometheus rules linter",
				Result: "PASS",
			},
			errorHandler: func(err error) error {
				if err != nil {
					return fmt.Errorf("Unpexpected error: %w", err)
				}
				return nil
			},
		},
		{
			description: "ignores failures from unmodified lines",
			gitCmd: func(args ...string) ([]byte, error) {
				if args[0] == "rev-parse" {
					return []byte("fake-commit-id"), nil
				}
				if args[0] == "blame" {
					content := blameLine("fake-commit-id", 4, "foo.txt", "errors")
					return []byte(content), nil
				}
				return nil, nil
			},
			summary: reporter.Summary{
				Reports: []reporter.Report{
					{
						Path: "foo.txt",
						Rule: mockRules[1],
						Problem: checks.Problem{
							Fragment: "up",
							Lines:    []int{1},
							Reporter: "mock",
							Text:     "this should be ignored, line is not part of the diff",
							Severity: checks.Bug,
						},
					},
					{
						Path: "bar.txt",
						Rule: mockRules[1],
						Problem: checks.Problem{
							Fragment: "up",
							Lines:    []int{1},
							Reporter: "mock",
							Text:     "this should be ignored, file is not part of the diff",
							Severity: checks.Bug,
						},
					},
					{
						Path: "foo.txt",
						Rule: mockRules[1],
						Problem: checks.Problem{
							Fragment: "up",
							Lines:    []int{2},
							Reporter: "mock",
							Text:     "bad name",
							Severity: checks.Bug,
						},
					},
					{
						Path: "foo.txt",
						Rule: mockRules[0],
						Problem: checks.Problem{
							Fragment: "up == 0",
							Lines:    []int{2},
							Reporter: "mock",
							Text:     "mock text",
							Severity: checks.Bug,
						},
					},
					{
						Path: "foo.txt",
						Rule: mockRules[1],
						Problem: checks.Problem{
							Fragment: "errors",
							Lines:    []int{4},
							Reporter: "mock",
							Text:     "mock text 2",
							Severity: checks.Warning,
						},
					},
				},
				FileChanges: discovery.NewFileCommitsFromMap(map[string][]string{"foo.txt": {"fake-commit-id"}}),
			},
			report: reporter.BitBucketReport{
				Title:  "Pint - Prometheus rules linter",
				Result: "PASS",
			},
			annotations: reporter.BitBucketAnnotations{
				Annotations: []reporter.BitBucketAnnotation{
					{
						Path:     "foo.txt",
						Line:     4,
						Message:  "mock: mock text 2",
						Severity: "LOW",
						Type:     "CODE_SMELL",
					},
				},
			},
			errorHandler: func(err error) error {
				if err != nil {
					return fmt.Errorf("Unpexpected error: %w", err)
				}
				return nil
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			var srv *httptest.Server
			if tc.httpHandler == nil {
				srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					defer r.Body.Close()

					if r.Method == http.MethodDelete {
						w.WriteHeader(200)
						return
					}

					switch r.URL.Path {
					case "/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint":
						var resp reporter.BitBucketReport
						if err := json.NewDecoder(r.Body).Decode(&resp); err != nil {
							t.Errorf("JSON decode error: %v", err)
						}
						if diff := cmp.Diff(tc.report, resp); diff != "" {
							t.Errorf("Got wrong bitbucket report body (-want +got):\n%s", diff)
							return
						}
					case "/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint/annotations":
						var resp reporter.BitBucketAnnotations
						if err := json.NewDecoder(r.Body).Decode(&resp); err != nil {
							t.Errorf("JSON decode error: %s", err)
						}
						if diff := cmp.Diff(tc.annotations, resp); diff != "" {
							t.Errorf("Got wrong bitbucket annotations (-want +got):\n%s", diff)
							return
						}
					default:
						w.WriteHeader(500)
						_, _ = w.Write([]byte(fmt.Sprintf("Unhandled path: %s", r.URL.Path)))
						t.Errorf("Unhandled path: %s", r.URL.Path)
					}
				}))
			} else {
				srv = httptest.NewServer(tc.httpHandler)
			}
			defer srv.Close()

			r := reporter.NewBitBucketReporter(
				srv.URL,
				time.Second,
				"token",
				"proj",
				"repo",
				tc.gitCmd)
			err := r.Submit(tc.summary)

			if e := tc.errorHandler(err); e != nil {
				t.Errorf("error check failure: %s", e)
				return
			}
		})
	}
}
