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

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/checks"
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
		reports      []reporter.Report
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
			reports: []reporter.Report{{Path: "foo.txt", Rule: mockRules[0], Problem: checks.Problem{}}},
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
			reports: []reporter.Report{
				{
					Path:          "foo.txt",
					ModifiedLines: []int{2},
					Rule:          mockRules[0],
					Problem:       checks.Problem{},
				},
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
			reports: []reporter.Report{
				{
					Path:          "foo.txt",
					ModifiedLines: []int{2},
					Rule:          mockRules[0],
					Problem:       checks.Problem{},
				},
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
			reports: []reporter.Report{
				{
					Path:          "foo.txt",
					ModifiedLines: []int{2},
					Rule:          mockRules[0],
					Problem:       checks.Problem{},
				},
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
			reports: []reporter.Report{
				{
					Path:          "foo.txt",
					ModifiedLines: []int{2, 4},
					Rule:          mockRules[1],
					Problem: checks.Problem{
						Fragment: "up",
						Lines:    []int{1},
						Reporter: "mock",
						Text:     "this should be ignored, line is not part of the diff",
						Severity: checks.Bug,
					},
				},
				{
					Path:          "bar.txt",
					ModifiedLines: []int{},
					Rule:          mockRules[1],
					Problem: checks.Problem{
						Fragment: "up",
						Lines:    []int{1},
						Reporter: "mock",
						Text:     "this should be ignored, file is not part of the diff",
						Severity: checks.Bug,
					},
				},
				{
					Path:          "foo.txt",
					ModifiedLines: []int{2, 4},
					Rule:          mockRules[1],
					Problem: checks.Problem{
						Fragment: "up",
						Lines:    []int{2},
						Reporter: "mock",
						Text:     "bad name",
						Severity: checks.Fatal,
					},
				},
				{
					Path:          "foo.txt",
					ModifiedLines: []int{2, 4},
					Rule:          mockRules[0],
					Problem: checks.Problem{
						Fragment: "up == 0",
						Lines:    []int{2},
						Reporter: "mock",
						Text:     "mock text",
						Severity: checks.Bug,
					},
				},
				{
					Path:          "foo.txt",
					ModifiedLines: []int{2, 4},
					Rule:          mockRules[1],
					Problem: checks.Problem{
						Fragment: "errors",
						Lines:    []int{4},
						Reporter: "mock",
						Text:     "mock text 2",
						Severity: checks.Warning,
					},
				},
			},
			report: reporter.BitBucketReport{
				Reporter: "Prometheus rule linter",
				Title:    "pint v0.0.0",
				Details:  reporter.BitBucketDescription,
				Link:     "https://cloudflare.github.io/pint/",
				Result:   "FAIL",
				Data: []reporter.BitBucketReportData{
					{Title: "Number of rules checked", Type: reporter.NumberType, Value: float64(0)},
					{Title: "Number of problems found", Type: reporter.NumberType, Value: float64(3)},
					{Title: "Number of offline checks", Type: reporter.NumberType, Value: float64(0)},
					{Title: "Number of online checks", Type: reporter.NumberType, Value: float64(0)},
					{Title: "Checks duration", Type: reporter.DurationType, Value: float64(0)},
				},
			},
			annotations: reporter.BitBucketAnnotations{
				Annotations: []reporter.BitBucketAnnotation{
					{
						Path:     "foo.txt",
						Line:     2,
						Message:  "mock: bad name",
						Severity: "HIGH",
						Type:     "BUG",
						Link:     "https://cloudflare.github.io/pint/checks/mock.html",
					},
					{
						Path:     "foo.txt",
						Line:     2,
						Message:  "mock: mock text",
						Severity: "MEDIUM",
						Type:     "BUG",
						Link:     "https://cloudflare.github.io/pint/checks/mock.html",
					},
					{
						Path:     "foo.txt",
						Line:     4,
						Message:  "mock: mock text 2",
						Severity: "LOW",
						Type:     "CODE_SMELL",
						Link:     "https://cloudflare.github.io/pint/checks/mock.html",
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
					content := blameLine("fake-commit-00", 1, "foo.txt", "ignore") +
						blameLine("fake-commit-00", 2, "foo.txt", "ignore") +
						blameLine("fake-commit-id", 3, "foo.txt", "ok") +
						blameLine("fake-commit-id", 4, "foo.txt", "syntax error") +
						blameLine("fake-commit-01", 5, "foo.txt", "ignore")
					return []byte(content), nil
				}
				return nil, nil
			},
			reports: []reporter.Report{
				{
					Path:          "foo.txt",
					ModifiedLines: []int{3, 4},
					Rule:          mockRules[1],
					Problem: checks.Problem{
						Fragment: "syntax error",
						Lines:    []int{1},
						Reporter: "test/mock",
						Text:     "syntax error",
						Severity: checks.Fatal,
					},
				},
			},
			report: reporter.BitBucketReport{
				Reporter: "Prometheus rule linter",
				Title:    "pint v0.0.0",
				Details:  reporter.BitBucketDescription,
				Link:     "https://cloudflare.github.io/pint/",
				Result:   "FAIL",
				Data: []reporter.BitBucketReportData{
					{Title: "Number of rules checked", Type: reporter.NumberType, Value: float64(0)},
					{Title: "Number of problems found", Type: reporter.NumberType, Value: float64(1)},
					{Title: "Number of offline checks", Type: reporter.NumberType, Value: float64(0)},
					{Title: "Number of online checks", Type: reporter.NumberType, Value: float64(0)},
					{Title: "Checks duration", Type: reporter.DurationType, Value: float64(0)},
				},
			},
			annotations: reporter.BitBucketAnnotations{
				Annotations: []reporter.BitBucketAnnotation{
					{
						Path:     "foo.txt",
						Line:     3,
						Message:  "test/mock: syntax error",
						Severity: "HIGH",
						Type:     "BUG",
						Link:     "https://cloudflare.github.io/pint/checks/test/mock.html",
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
			report: reporter.BitBucketReport{
				Reporter: "Prometheus rule linter",
				Title:    "pint v0.0.0",
				Details:  reporter.BitBucketDescription,
				Link:     "https://cloudflare.github.io/pint/",
				Result:   "PASS",
				Data: []reporter.BitBucketReportData{
					{Title: "Number of rules checked", Type: reporter.NumberType, Value: float64(0)},
					{Title: "Number of problems found", Type: reporter.NumberType, Value: float64(0)},
					{Title: "Number of offline checks", Type: reporter.NumberType, Value: float64(0)},
					{Title: "Number of online checks", Type: reporter.NumberType, Value: float64(0)},
					{Title: "Checks duration", Type: reporter.DurationType, Value: float64(0)},
				},
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
			reports: []reporter.Report{
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
			report: reporter.BitBucketReport{
				Reporter: "Prometheus rule linter",
				Title:    "pint v0.0.0",
				Details:  reporter.BitBucketDescription,
				Link:     "https://cloudflare.github.io/pint/",
				Result:   "PASS",
				Data: []reporter.BitBucketReportData{
					{Title: "Number of rules checked", Type: reporter.NumberType, Value: float64(0)},
					{Title: "Number of problems found", Type: reporter.NumberType, Value: float64(0)},
					{Title: "Number of offline checks", Type: reporter.NumberType, Value: float64(0)},
					{Title: "Number of online checks", Type: reporter.NumberType, Value: float64(0)},
					{Title: "Checks duration", Type: reporter.DurationType, Value: float64(0)},
				},
			},
			annotations: reporter.BitBucketAnnotations{
				Annotations: []reporter.BitBucketAnnotation{
					{
						Path:     "foo.txt",
						Line:     4,
						Message:  "mock: mock text 2",
						Severity: "LOW",
						Type:     "CODE_SMELL",
						Link:     "https://cloudflare.github.io/pint/checks/mock.html",
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
						require.Equal(t, tc.report, resp, "Got wrong bitbucket report body")
					case "/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint/annotations":
						var resp reporter.BitBucketAnnotations
						if err := json.NewDecoder(r.Body).Decode(&resp); err != nil {
							t.Errorf("JSON decode error: %s", err)
						}
						require.Equal(t, tc.annotations, resp, "Got wrong bitbucket annotations")
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
				"v0.0.0",
				srv.URL,
				time.Second,
				"token",
				"proj",
				"repo",
				tc.gitCmd)
			summary := reporter.NewSummary(tc.reports)
			err := r.Submit(summary)

			if e := tc.errorHandler(err); e != nil {
				t.Errorf("error check failure: %s", e)
				return
			}
		})
	}
}
