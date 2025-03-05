package reporter_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/neilotoole/slogt"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/git"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/reporter"
)

func TestBitBucketReporter(t *testing.T) {
	type errorCheck func(err error) error

	type testCaseT struct {
		httpHandler           http.Handler
		gitCmd                git.CommandRunner
		pullRequestFileDiffs  map[string]reporter.BitBucketFileDiffs
		errorHandler          errorCheck
		description           string
		report                reporter.BitBucketReport
		reports               []reporter.Report
		annotations           reporter.BitBucketAnnotations
		pullRequestComments   []reporter.BitBucketPendingComment
		pullRequests          reporter.BitBucketPullRequests
		pullRequestChanges    reporter.BitBucketPullRequestChanges
		pullRequestActivities reporter.BitBucketPullRequestActivities
	}

	p := parser.NewParser(false, parser.PrometheusSchema, model.UTF8Validation)
	mockRules, _ := p.Parse([]byte(`
- record: target is down
  expr: up == 0
- record: sum errors
  expr: sum(errors) by (job)
`))

	fakeGit := func(args ...string) ([]byte, error) {
		if args[0] == "rev-parse" && args[1] == "--verify" && args[2] == "HEAD" {
			return []byte("fake-commit-id"), nil
		}
		if args[0] == "rev-parse" && args[1] == "--abbrev-ref" && args[2] == "HEAD" {
			return []byte("fake-branch"), nil
		}
		return nil, nil
	}

	emptyReport := reporter.BitBucketReport{
		Reporter: "Prometheus rule linter",
		Title:    "pint v0.0.0",
		Details:  reporter.BitBucketDescription,
		Link:     "https://cloudflare.github.io/pint/",
		Result:   "PASS",
		Data: []reporter.BitBucketReportData{
			{Title: "Number of rules parsed", Type: reporter.NumberType, Value: float64(0)},
			{Title: "Number of rules checked", Type: reporter.NumberType, Value: float64(0)},
			{Title: "Number of problems found", Type: reporter.NumberType, Value: float64(0)},
			{Title: "Number of offline checks", Type: reporter.NumberType, Value: float64(0)},
			{Title: "Number of online checks", Type: reporter.NumberType, Value: float64(0)},
			{Title: "Checks duration", Type: reporter.DurationType, Value: float64(0)},
		},
	}

	testCases := []testCaseT{
		{
			description: "returns an error on git head failure",
			gitCmd: func(args ...string) ([]byte, error) {
				if args[0] == "rev-parse" && args[1] == "--verify" && args[2] == "HEAD" {
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
			description: "returns an error on git branch failure",
			gitCmd: func(args ...string) ([]byte, error) {
				if args[0] == "rev-parse" && args[1] == "--verify" && args[2] == "HEAD" {
					return []byte("fake-commit-id"), nil
				}
				if args[0] == "rev-parse" && args[1] == "--abbrev-ref" && args[2] == "HEAD" {
					return nil, errors.New("git branch error")
				}
				return nil, nil
			},
			errorHandler: func(err error) error {
				if err != nil && err.Error() == "failed to get current branch: git branch error" {
					return nil
				}
				return fmt.Errorf("Expected git branch error, got %w", err)
			},
			report: emptyReport,
		},
		{
			description: "returns an error on non-200 HTTP response",
			gitCmd:      fakeGit,
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
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte("Bad Request"))
			}),
			errorHandler: func(err error) error {
				if err != nil && err.Error() == "failed to create BitBucket report: PUT request failed" {
					return nil
				}
				return fmt.Errorf("Expected 'failed to create BitBucket report: PUT request failed', got %w", err)
			},
		},
		{
			description: "returns an error on HTTP response headers timeout",
			gitCmd:      fakeGit,
			report:      emptyReport,
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
				w.WriteHeader(http.StatusBadRequest)
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
			gitCmd:      fakeGit,
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
				w.WriteHeader(http.StatusBadRequest)
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
			description: "sends a correct report that fails",
			gitCmd:      fakeGit,
			report:      emptyReport,
			httpHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodDelete {
					w.WriteHeader(http.StatusOK)
					return
				}
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte("Internal error"))
			}),
			errorHandler: func(err error) error {
				if err.Error() != "failed to create BitBucket report: PUT request failed" {
					return fmt.Errorf("Unpexpected error: %w", err)
				}
				return nil
			},
		},
		{
			description: "sends a correct report but fails to delete annotations",
			gitCmd:      fakeGit,
			report:      emptyReport,
			httpHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint" {
					w.WriteHeader(http.StatusOK)
					return
				}
				if r.URL.Path == "/rest/api/1.0/projects/proj/repos/repo/commits/fake-commit-id/pull-requests" {
					data, err := json.Marshal(reporter.BitBucketPullRequests{
						IsLastPage: true,
						Values:     []reporter.BitBucketPullRequest{},
					})
					assert.NoError(t, err)
					w.WriteHeader(http.StatusOK)
					_, err = w.Write(data)
					assert.NoError(t, err)
					return
				}
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte("Internal error"))
			}),
			errorHandler: func(err error) error {
				if err.Error() != "failed to delete existing BitBucket code insight annotations: DELETE request failed" {
					return fmt.Errorf("Unpexpected error: %w", err)
				}
				return nil
			},
		},
		{
			description: "sends a correct report but fails to create annotations",
			gitCmd:      fakeGit,
			reports: []reporter.Report{
				{
					Path: discovery.Path{
						SymlinkTarget: "foo.txt",
						Name:          "foo.txt",
					},
					ModifiedLines: []int{2, 4},
					Rule:          mockRules[1],
					Problem: checks.Problem{
						Lines: parser.LineRange{
							First: 1,
							Last:  1,
						},
						Reporter: "mock",
						Summary:  "this should be ignored, line is not part of the diff",
						Severity: checks.Bug,
					},
				},
				{
					Path: discovery.Path{
						SymlinkTarget: "bar.txt",
						Name:          "bar.txt",
					},
					ModifiedLines: []int{},
					Rule:          mockRules[1],
					Problem: checks.Problem{
						Lines: parser.LineRange{
							First: 1,
							Last:  1,
						},
						Reporter: "mock",
						Summary:  "this should be ignored, file is not part of the diff",
						Severity: checks.Bug,
					},
				},
				{
					Path: discovery.Path{
						SymlinkTarget: "foo.txt",
						Name:          "foo.txt",
					},
					ModifiedLines: []int{2, 4},
					Rule:          mockRules[1],
					Problem: checks.Problem{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "mock",
						Summary:  "bad name",
						Severity: checks.Fatal,
					},
				},
				{
					Path: discovery.Path{
						SymlinkTarget: "foo.txt",
						Name:          "foo.txt",
					},
					ModifiedLines: []int{2, 4},
					Rule:          mockRules[0],
					Problem: checks.Problem{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "mock",
						Summary:  "mock text",
						Severity: checks.Bug,
					},
				},
				{
					Path: discovery.Path{
						SymlinkTarget: "foo.txt",
						Name:          "foo.txt",
					},
					ModifiedLines: []int{2, 4},
					Rule:          mockRules[1],
					Problem: checks.Problem{
						Lines: parser.LineRange{
							First: 4,
							Last:  4,
						},
						Reporter: "mock",
						Summary:  "mock text 2",
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
					{Title: "Number of rules parsed", Type: reporter.NumberType, Value: float64(0)},
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
			httpHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint" {
					w.WriteHeader(http.StatusOK)
					return
				}
				if r.URL.Path == "/rest/api/1.0/projects/proj/repos/repo/commits/fake-commit-id/pull-requests" {
					data, err := json.Marshal(reporter.BitBucketPullRequests{
						IsLastPage: true,
						Values:     []reporter.BitBucketPullRequest{},
					})
					assert.NoError(t, err)
					w.WriteHeader(http.StatusOK)
					_, err = w.Write(data)
					assert.NoError(t, err)
					return
				}
				if r.Method == http.MethodDelete {
					w.WriteHeader(http.StatusOK)
					return
				}
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte("Internal error"))
			}),
			errorHandler: func(err error) error {
				if err.Error() != "failed to create BitBucket code insight annotations: POST request failed" {
					return fmt.Errorf("Unpexpected error: %w", err)
				}
				return nil
			},
		},
		{
			description: "pull requests get fails",
			gitCmd:      fakeGit,
			report:      emptyReport,
			httpHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					w.WriteHeader(http.StatusOK)
					return
				}
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte("Internal error"))
			}),
			errorHandler: func(err error) error {
				if err.Error() != "failed to get open pull requests from BitBucket: GET request failed" {
					return fmt.Errorf("Unpexpected error: %w", err)
				}
				return nil
			},
		},
		{
			description: "pull request changes get fails",
			gitCmd:      fakeGit,
			report:      emptyReport,
			httpHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/rest/api/1.0/projects/proj/repos/repo/commits/fake-commit-id/pull-requests" {
					data, err := json.Marshal(reporter.BitBucketPullRequests{
						IsLastPage: true,
						Values: []reporter.BitBucketPullRequest{
							{
								ID:   102,
								Open: true,
								FromRef: reporter.BitBucketRef{
									ID:     "refs/heads/fake-branch",
									Commit: "fake-commit-id",
								},
								ToRef: reporter.BitBucketRef{
									ID:     "refs/heads/main",
									Commit: "main-commit-id",
								},
							},
						},
					})
					assert.NoError(t, err)
					w.WriteHeader(http.StatusOK)
					_, err = w.Write(data)
					assert.NoError(t, err)
				}
				if strings.HasSuffix(r.URL.Path, "/changes") {
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte("Internal error"))
					return
				}
				w.WriteHeader(http.StatusOK)
			}),
			errorHandler: func(err error) error {
				if err.Error() != "failed to get pull request changes from BitBucket: GET request failed" {
					return fmt.Errorf("Unpexpected error: %w", err)
				}
				return nil
			},
		},
		{
			description: "pull request comments get fails",
			gitCmd:      fakeGit,
			report:      emptyReport,
			httpHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/rest/api/1.0/projects/proj/repos/repo/commits/fake-commit-id/pull-requests" {
					data, err := json.Marshal(reporter.BitBucketPullRequests{
						IsLastPage: true,
						Values: []reporter.BitBucketPullRequest{
							{
								ID:   102,
								Open: true,
								FromRef: reporter.BitBucketRef{
									ID:     "refs/heads/fake-branch",
									Commit: "fake-commit-id",
								},
								ToRef: reporter.BitBucketRef{
									ID:     "refs/heads/main",
									Commit: "main-commit-id",
								},
							},
						},
					})
					assert.NoError(t, err)
					w.WriteHeader(http.StatusOK)
					_, err = w.Write(data)
					assert.NoError(t, err)
				}
				if r.URL.Path == "/rest/api/1.0/projects/proj/repos/repo/pull-requests/102/changes" {
					data, err := json.Marshal(reporter.BitBucketPullRequestChanges{
						IsLastPage: true,
						Values:     []reporter.BitBucketPullRequestChange{},
					})
					assert.NoError(t, err)
					w.WriteHeader(http.StatusOK)
					_, err = w.Write(data)
					assert.NoError(t, err)
				}
				if r.URL.Path == "/rest/api/latest/projects/proj/repos/repo/pull-requests/102/activities" {
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte("Internal error"))
					return
				}
				w.WriteHeader(http.StatusOK)
			}),
			errorHandler: func(err error) error {
				if err.Error() != "failed to get pull request comments from BitBucket: GET request failed" {
					return fmt.Errorf("Unpexpected error: %w", err)
				}
				return nil
			},
		},
		{
			description: "sends a correct report",
			gitCmd:      fakeGit,
			reports: []reporter.Report{
				{
					Path: discovery.Path{
						SymlinkTarget: "foo.txt",
						Name:          "foo.txt",
					},
					ModifiedLines: []int{2, 4},
					Rule:          mockRules[1],
					Problem: checks.Problem{
						Lines: parser.LineRange{
							First: 1,
							Last:  1,
						},
						Reporter: "mock",
						Summary:  "line is not part of the diff",
						Severity: checks.Bug,
					},
				},
				{
					Path: discovery.Path{
						SymlinkTarget: "foo.txt",
						Name:          "foo.txt",
					},
					ModifiedLines: []int{2, 4},
					Rule:          mockRules[1],
					Problem: checks.Problem{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "mock",
						Summary:  "bad name",
						Severity: checks.Fatal,
					},
				},
				{
					Path: discovery.Path{
						SymlinkTarget: "foo.txt",
						Name:          "foo.txt",
					},
					ModifiedLines: []int{2, 4},
					Rule:          mockRules[0],
					Problem: checks.Problem{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "mock",
						Summary:  "mock text",
						Severity: checks.Bug,
					},
				},
				{
					Path: discovery.Path{
						SymlinkTarget: "foo.txt",
						Name:          "foo.txt",
					},
					ModifiedLines: []int{2, 4},
					Rule:          mockRules[1],
					Problem: checks.Problem{
						Lines: parser.LineRange{
							First: 4,
							Last:  4,
						},
						Reporter: "mock",
						Summary:  "mock text 2",
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
					{Title: "Number of rules parsed", Type: reporter.NumberType, Value: float64(0)},
					{Title: "Number of rules checked", Type: reporter.NumberType, Value: float64(0)},
					{Title: "Number of problems found", Type: reporter.NumberType, Value: float64(4)},
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
						Message:  "Problem reported on unmodified line 1, annotation moved here: mock: line is not part of the diff",
						Severity: "MEDIUM",
						Type:     "BUG",
						Link:     "https://cloudflare.github.io/pint/checks/mock.html",
					},
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
			gitCmd:      fakeGit,
			reports: []reporter.Report{
				{
					Path: discovery.Path{
						SymlinkTarget: "foo.txt",
						Name:          "foo.txt",
					},
					ModifiedLines: []int{3, 4},
					Rule:          mockRules[1],
					Problem: checks.Problem{
						Lines: parser.LineRange{
							First: 1,
							Last:  1,
						},
						Reporter: "test/mock",
						Summary:  "syntax error",
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
					{Title: "Number of rules parsed", Type: reporter.NumberType, Value: float64(0)},
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
						Message:  "Problem reported on unmodified line 1, annotation moved here: test/mock: syntax error",
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
			gitCmd:      fakeGit,
			report:      emptyReport,
			errorHandler: func(err error) error {
				if err != nil {
					return fmt.Errorf("Unpexpected error: %w", err)
				}
				return nil
			},
		},
		{
			description: "reports failures from unmodified lines",
			gitCmd:      fakeGit,
			reports: []reporter.Report{
				{
					Path: discovery.Path{
						SymlinkTarget: "foo.txt",
						Name:          "foo.txt",
					},
					Rule:          mockRules[1],
					ModifiedLines: []int{2, 4},
					Problem: checks.Problem{
						Lines: parser.LineRange{
							First: 1,
							Last:  1,
						},
						Reporter: "mock",
						Summary:  "this line is not part of the diff",
						Severity: checks.Bug,
					},
				},
				{
					Path: discovery.Path{
						SymlinkTarget: "foo.txt",
						Name:          "foo.txt",
					},
					Rule:          mockRules[1],
					ModifiedLines: []int{2, 4},
					Problem: checks.Problem{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "mock",
						Summary:  "bad name",
						Severity: checks.Bug,
					},
				},
				{
					Path: discovery.Path{
						SymlinkTarget: "foo.txt",
						Name:          "foo.txt",
					},
					Rule:          mockRules[0],
					ModifiedLines: []int{2, 4},
					Problem: checks.Problem{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "mock",
						Summary:  "mock text",
						Severity: checks.Bug,
					},
				},
				{
					Path: discovery.Path{
						SymlinkTarget: "foo.txt",
						Name:          "foo.txt",
					},
					Rule:          mockRules[1],
					ModifiedLines: []int{2, 4},
					Problem: checks.Problem{
						Lines: parser.LineRange{
							First: 4,
							Last:  4,
						},
						Reporter: "mock",
						Summary:  "mock text 2",
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
					{Title: "Number of rules parsed", Type: reporter.NumberType, Value: float64(0)},
					{Title: "Number of rules checked", Type: reporter.NumberType, Value: float64(0)},
					{Title: "Number of problems found", Type: reporter.NumberType, Value: float64(4)},
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
						Message:  "Problem reported on unmodified line 1, annotation moved here: mock: this line is not part of the diff",
						Severity: "MEDIUM",
						Type:     "BUG",
						Link:     "https://cloudflare.github.io/pint/checks/mock.html",
					},
					{
						Path:     "foo.txt",
						Line:     2,
						Message:  "mock: bad name",
						Severity: "MEDIUM",
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
				if err != nil {
					return fmt.Errorf("Unpexpected error: %w", err)
				}
				return nil
			},
		},
		{
			description: "sends a correct report with pull request open",
			gitCmd:      fakeGit,
			report:      emptyReport,
			pullRequests: reporter.BitBucketPullRequests{
				IsLastPage: true,
				Values: []reporter.BitBucketPullRequest{
					{
						ID:   101,
						Open: false,
						FromRef: reporter.BitBucketRef{
							ID:     "refs/heads/feature",
							Commit: "pr-commit-id",
						},
						ToRef: reporter.BitBucketRef{
							ID:     "refs/heads/main",
							Commit: "main-commit-id",
						},
					},
					{
						ID:   102,
						Open: true,
						FromRef: reporter.BitBucketRef{
							ID:     "refs/heads/fake-branch",
							Commit: "fake-commit-id",
						},
						ToRef: reporter.BitBucketRef{
							ID:     "refs/heads/main",
							Commit: "main-commit-id",
						},
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
		{
			description: "sends a correct report using comments, deleting stale ones",
			gitCmd:      fakeGit,
			reports: []reporter.Report{
				{
					Path: discovery.Path{
						SymlinkTarget: "foo.txt",
						Name:          "foo.txt",
					},
					ModifiedLines: []int{2, 4},
					Rule:          mockRules[1],
					Problem: checks.Problem{
						Lines: parser.LineRange{
							First: 1,
							Last:  1,
						},
						Reporter: "mock",
						Summary:  "this should be ignored, line is not part of the diff",
						Severity: checks.Bug,
					},
				},
				{
					Path: discovery.Path{
						SymlinkTarget: "foo.txt",
						Name:          "foo.txt",
					},
					ModifiedLines: []int{2, 4},
					Rule:          mockRules[1],
					Problem: checks.Problem{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "mock",
						Summary:  "bad name",
						Severity: checks.Fatal,
					},
				},
				{
					Path: discovery.Path{
						SymlinkTarget: "foo.txt",
						Name:          "foo.txt",
					},
					ModifiedLines: []int{2, 4},
					Rule:          mockRules[0],
					Problem: checks.Problem{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "mock",
						Summary:  "mock text",
						Details:  "mock details",
						Severity: checks.Bug,
					},
				},
				{
					Path: discovery.Path{
						SymlinkTarget: "foo.txt",
						Name:          "symlink.txt",
					},
					ModifiedLines: []int{2, 4},
					Rule:          mockRules[1],
					Problem: checks.Problem{
						Lines: parser.LineRange{
							First: 4,
							Last:  4,
						},
						Reporter: "mock",
						Summary:  "mock text 2",
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
					{Title: "Number of rules parsed", Type: reporter.NumberType, Value: float64(0)},
					{Title: "Number of rules checked", Type: reporter.NumberType, Value: float64(0)},
					{Title: "Number of problems found", Type: reporter.NumberType, Value: float64(4)},
					{Title: "Number of offline checks", Type: reporter.NumberType, Value: float64(0)},
					{Title: "Number of online checks", Type: reporter.NumberType, Value: float64(0)},
					{Title: "Checks duration", Type: reporter.DurationType, Value: float64(0)},
				},
			},
			pullRequestComments: []reporter.BitBucketPendingComment{
				{
					Text:     ":stop_sign: **Bug** reported by [pint](https://cloudflare.github.io/pint/) **mock** check.\n\n------\n\nthis should be ignored, line is not part of the diff\n\n------\n\n:information_source: To see documentation covering this check and instructions on how to resolve it [click here](https://cloudflare.github.io/pint/checks/mock.html).\n",
					Severity: "BLOCKER",
					Anchor: reporter.BitBucketPendingCommentAnchor{
						Path:     "foo.txt",
						Line:     1,
						LineType: "CONTEXT",
						FileType: "FROM",
						DiffType: "EFFECTIVE",
					},
				},
				{
					Text:     ":stop_sign: **Fatal** reported by [pint](https://cloudflare.github.io/pint/) **mock** check.\n\n------\n\nbad name\n\n------\n\n:information_source: To see documentation covering this check and instructions on how to resolve it [click here](https://cloudflare.github.io/pint/checks/mock.html).\n",
					Severity: "BLOCKER",
					Anchor: reporter.BitBucketPendingCommentAnchor{
						Path:     "foo.txt",
						Line:     2,
						LineType: "ADDED",
						FileType: "TO",
						DiffType: "EFFECTIVE",
					},
				},
				{
					Text:     ":stop_sign: **Bug** reported by [pint](https://cloudflare.github.io/pint/) **mock** check.\n\n------\n\nmock text\n\nmock details\n\n------\n\n:information_source: To see documentation covering this check and instructions on how to resolve it [click here](https://cloudflare.github.io/pint/checks/mock.html).\n",
					Severity: "BLOCKER",
					Anchor: reporter.BitBucketPendingCommentAnchor{
						Path:     "foo.txt",
						Line:     2,
						LineType: "ADDED",
						FileType: "TO",
						DiffType: "EFFECTIVE",
					},
				},
				{
					Text:     ":warning: **Warning** reported by [pint](https://cloudflare.github.io/pint/) **mock** check.\n\n------\n\nmock text 2\n\n:leftwards_arrow_with_hook: This problem was detected on a symlinked file `symlink.txt`.\n\n------\n\n:information_source: To see documentation covering this check and instructions on how to resolve it [click here](https://cloudflare.github.io/pint/checks/mock.html).\n",
					Severity: "NORMAL",
					Anchor: reporter.BitBucketPendingCommentAnchor{
						Path:     "foo.txt",
						Line:     4,
						LineType: "CONTEXT",
						FileType: "FROM",
						DiffType: "EFFECTIVE",
					},
				},
			},
			pullRequests: reporter.BitBucketPullRequests{
				IsLastPage: true,
				Values: []reporter.BitBucketPullRequest{
					{
						ID:   102,
						Open: true,
						FromRef: reporter.BitBucketRef{
							ID:     "refs/heads/fake-branch",
							Commit: "fake-commit-id",
						},
						ToRef: reporter.BitBucketRef{
							ID:     "refs/heads/main",
							Commit: "main-commit-id",
						},
					},
				},
			},
			pullRequestChanges: reporter.BitBucketPullRequestChanges{
				IsLastPage: true,
				Values: []reporter.BitBucketPullRequestChange{
					{
						Path: reporter.BitBucketPath{
							ToString: "index.txt",
						},
					},
					{
						Path: reporter.BitBucketPath{
							ToString: "foo.txt",
						},
					},
				},
			},
			pullRequestFileDiffs: map[string]reporter.BitBucketFileDiffs{
				"index.txt": {
					Diffs: []reporter.BitBucketFileDiff{
						{
							Hunks: []reporter.BitBucketDiffHunk{
								{
									Segments: []reporter.BitBucketDiffSegment{
										{
											Type: "ADDED",
											Lines: []reporter.BitBucketDiffLine{
												{Source: 1, Destination: 1},
												{Source: 5, Destination: 5},
											},
										},
										{
											Type: "CONTEXT",
											Lines: []reporter.BitBucketDiffLine{
												{Source: 10, Destination: 6},
											},
										},
									},
								},
							},
						},
					},
				},
				"foo.txt": {
					Diffs: []reporter.BitBucketFileDiff{
						{
							Hunks: []reporter.BitBucketDiffHunk{
								{
									Segments: []reporter.BitBucketDiffSegment{
										{
											Type: "ADDED",
											Lines: []reporter.BitBucketDiffLine{
												{Source: 2, Destination: 2},
											},
										},
									},
								},
							},
						},
						{
							Hunks: []reporter.BitBucketDiffHunk{
								{
									Segments: []reporter.BitBucketDiffSegment{
										{
											Type: "MODIFIED",
											Lines: []reporter.BitBucketDiffLine{
												{Source: 3, Destination: 4},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			pullRequestActivities: reporter.BitBucketPullRequestActivities{
				IsLastPage: true,
				Values: []reporter.BitBucketPullRequestActivity{
					{
						Action: "APPROVED",
					},
					{
						Action:        "COMMENTED",
						CommentAction: "ADDED",
						CommentAnchor: reporter.BitBucketCommentAnchor{
							Orphaned: true,
							LineType: "CONTEXT",
							DiffType: "EFFECTIVE",
							Path:     "foo.txt",
							Line:     3,
						},
						Comment: reporter.BitBucketPullRequestComment{
							ID:      1001,
							Version: 0,
							State:   "OPEN",
							Author: reporter.BitBucketCommentAuthor{
								Name: "pint_user",
							},
						},
					},
					{
						Action:        "COMMENTED",
						CommentAction: "ADDED",
						CommentAnchor: reporter.BitBucketCommentAnchor{
							Orphaned: true,
							DiffType: "COMMIT",
							Path:     "foo.txt",
							Line:     10,
						},
						Comment: reporter.BitBucketPullRequestComment{
							ID:      1002,
							Version: 1,
							State:   "OPEN",
							Author: reporter.BitBucketCommentAuthor{
								Name: "pint_user",
							},
						},
					},
					{
						Action:        "COMMENTED",
						CommentAction: "ADDED",
						CommentAnchor: reporter.BitBucketCommentAnchor{
							Orphaned: true,
							LineType: "REMOVED",
							DiffType: "COMMIT",
							Path:     "foo.txt",
							Line:     14,
						},
						Comment: reporter.BitBucketPullRequestComment{
							ID:       1003,
							Version:  1,
							State:    "OPEN",
							Severity: "BLOCKER",
							Author: reporter.BitBucketCommentAuthor{
								Name: "pint_user",
							},
						},
					},
					{
						Action:        "COMMENTED",
						CommentAction: "ADDED",
						CommentAnchor: reporter.BitBucketCommentAnchor{
							Orphaned: false,
							DiffType: "EFFECTIVE",
							Path:     "foo.txt",
							Line:     3,
						},
						Comment: reporter.BitBucketPullRequestComment{
							ID:      2001,
							Version: 0,
							State:   "OPEN",
							Author: reporter.BitBucketCommentAuthor{
								Name: "pint_user",
							},
						},
					},
					{
						Action:        "COMMENTED",
						CommentAction: "ADDED",
						CommentAnchor: reporter.BitBucketCommentAnchor{
							Orphaned: false,
							DiffType: "COMMIT",
							Path:     "foo.txt",
							Line:     4,
						},
						Comment: reporter.BitBucketPullRequestComment{
							ID:      2002,
							Version: 1,
							State:   "OPEN",
							Author: reporter.BitBucketCommentAuthor{
								Name: "pint_user",
							},
						},
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
			description: "sends a correct report using comments, fails to delete stale comments",
			gitCmd:      fakeGit,
			report:      emptyReport,
			httpHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/rest/api/1.0/projects/proj/repos/repo/commits/fake-commit-id/pull-requests" {
					data, err := json.Marshal(reporter.BitBucketPullRequests{
						IsLastPage: true,
						Values: []reporter.BitBucketPullRequest{
							{
								ID:   102,
								Open: true,
								FromRef: reporter.BitBucketRef{
									ID:     "refs/heads/fake-branch",
									Commit: "fake-commit-id",
								},
								ToRef: reporter.BitBucketRef{
									ID:     "refs/heads/main",
									Commit: "main-commit-id",
								},
							},
						},
					})
					assert.NoError(t, err)
					w.WriteHeader(http.StatusOK)
					_, err = w.Write(data)
					assert.NoError(t, err)
				}
				if r.URL.Path == "/rest/api/1.0/projects/proj/repos/repo/pull-requests/102/changes" {
					data, err := json.Marshal(reporter.BitBucketPullRequestChanges{
						IsLastPage: true,
						Values: []reporter.BitBucketPullRequestChange{
							{
								Path: reporter.BitBucketPath{
									ToString: "index.txt",
								},
							},
						},
					})
					assert.NoError(t, err)
					w.WriteHeader(http.StatusOK)
					_, err = w.Write(data)
					assert.NoError(t, err)
				}
				if r.URL.Path == "/rest/api/latest/projects/proj/repos/repo/pull-requests/102/activities" {
					data, err := json.Marshal(reporter.BitBucketPullRequestActivities{
						IsLastPage: true,
						Values: []reporter.BitBucketPullRequestActivity{
							{
								Action:        "COMMENTED",
								CommentAction: "ADDED",
								CommentAnchor: reporter.BitBucketCommentAnchor{
									Orphaned: false,
									DiffType: "EFFECTIVE",
									Path:     "index.txt",
									Line:     3,
								},
								Comment: reporter.BitBucketPullRequestComment{
									ID:      1001,
									Version: 0,
									State:   "OPEN",
									Author: reporter.BitBucketCommentAuthor{
										Name: "pint_user",
									},
								},
							},
							{
								Action:        "COMMENTED",
								CommentAction: "ADDED",
								CommentAnchor: reporter.BitBucketCommentAnchor{
									Orphaned: false,
									DiffType: "COMMIT",
									Path:     "index.txt",
									Line:     10,
								},
								Comment: reporter.BitBucketPullRequestComment{
									ID:      1002,
									Version: 1,
									State:   "OPEN",
									Author: reporter.BitBucketCommentAuthor{
										Name: "pint_user",
									},
								},
							},
						},
					})
					assert.NoError(t, err)
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write(data)
					return
				}
				if r.URL.Path == "/rest/api/latest/projects/proj/repos/repo/commits/fake-commit-id/diff/index.txt" {
					data, err := json.Marshal(reporter.BitBucketFileDiffs{
						Diffs: []reporter.BitBucketFileDiff{
							{
								Hunks: []reporter.BitBucketDiffHunk{
									{
										Segments: []reporter.BitBucketDiffSegment{
											{
												Type: "ADDED",
												Lines: []reporter.BitBucketDiffLine{
													{Source: 1, Destination: 1},
													{Source: 5, Destination: 5},
												},
											},
											{
												Type: "CONTEXT",
												Lines: []reporter.BitBucketDiffLine{
													{Source: 10, Destination: 6},
												},
											},
										},
									},
								},
							},
						},
					})
					assert.NoError(t, err)
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write(data)
					return
				}
				if r.URL.Path == "/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint" {
					w.WriteHeader(http.StatusOK)
					return
				}
				if r.URL.Path == "/plugins/servlet/applinks/whoami" {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("pint_user"))
					return
				}
				w.WriteHeader(http.StatusInternalServerError)
			}),
			errorHandler: func(err error) error {
				if err != nil {
					return fmt.Errorf("Unpexpected error: %w", err)
				}
				return nil
			},
		},
		{
			description: "sends a correct report using comments, fails to get username",
			gitCmd:      fakeGit,
			report:      emptyReport,
			httpHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/rest/api/1.0/projects/proj/repos/repo/commits/fake-commit-id/pull-requests" {
					data, err := json.Marshal(reporter.BitBucketPullRequests{
						IsLastPage: true,
						Values: []reporter.BitBucketPullRequest{
							{
								ID:   102,
								Open: true,
								FromRef: reporter.BitBucketRef{
									ID:     "refs/heads/fake-branch",
									Commit: "fake-commit-id",
								},
								ToRef: reporter.BitBucketRef{
									ID:     "refs/heads/main",
									Commit: "main-commit-id",
								},
							},
						},
					})
					assert.NoError(t, err)
					w.WriteHeader(http.StatusOK)
					_, err = w.Write(data)
					assert.NoError(t, err)
				}
				if r.URL.Path == "/rest/api/1.0/projects/proj/repos/repo/pull-requests/102/changes" {
					data, err := json.Marshal(reporter.BitBucketPullRequestChanges{
						IsLastPage: true,
						Values: []reporter.BitBucketPullRequestChange{
							{
								Path: reporter.BitBucketPath{
									ToString: "index.txt",
								},
							},
						},
					})
					assert.NoError(t, err)
					w.WriteHeader(http.StatusOK)
					_, err = w.Write(data)
					assert.NoError(t, err)
				}
				if r.URL.Path == "/rest/api/latest/projects/proj/repos/repo/pull-requests/102/activities" {
					data, err := json.Marshal(reporter.BitBucketPullRequestActivities{
						IsLastPage: true,
						Values:     []reporter.BitBucketPullRequestActivity{},
					})
					assert.NoError(t, err)
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write(data)
					return
				}
				if r.URL.Path == "/rest/api/latest/projects/proj/repos/repo/commits/fake-commit-id/diff/index.txt" {
					data, err := json.Marshal(reporter.BitBucketFileDiffs{
						Diffs: []reporter.BitBucketFileDiff{
							{
								Hunks: []reporter.BitBucketDiffHunk{
									{
										Segments: []reporter.BitBucketDiffSegment{
											{
												Type: "ADDED",
												Lines: []reporter.BitBucketDiffLine{
													{Source: 1, Destination: 1},
													{Source: 5, Destination: 5},
												},
											},
											{
												Type: "CONTEXT",
												Lines: []reporter.BitBucketDiffLine{
													{Source: 10, Destination: 6},
												},
											},
										},
									},
								},
							},
						},
					})
					assert.NoError(t, err)
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write(data)
					return
				}
				if r.URL.Path == "/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint" {
					w.WriteHeader(http.StatusOK)
					return
				}
				if r.URL.Path == "/plugins/servlet/applinks/whoami" {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusOK)
			}),
			errorHandler: func(err error) error {
				if err.Error() != "failed to get pull request comments from BitBucket: GET request failed" {
					return fmt.Errorf("Unpexpected error: %w", err)
				}
				return nil
			},
		},
		{
			description: "sends a correct report using comments, fails to create new comments",
			gitCmd:      fakeGit,
			reports: []reporter.Report{
				{
					Path: discovery.Path{
						SymlinkTarget: "index.txt",
						Name:          "foo.txt",
					},
					ModifiedLines: []int{2, 4},
					Rule:          mockRules[1],
					Problem: checks.Problem{
						Lines: parser.LineRange{
							First: 1,
							Last:  1,
						},
						Reporter: "mock",
						Summary:  "this should be ignored, line is not part of the diff",
						Severity: checks.Bug,
					},
				},
			},
			report: emptyReport,
			httpHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/rest/api/1.0/projects/proj/repos/repo/commits/fake-commit-id/pull-requests" {
					data, err := json.Marshal(reporter.BitBucketPullRequests{
						IsLastPage: true,
						Values: []reporter.BitBucketPullRequest{
							{
								ID:   102,
								Open: true,
								FromRef: reporter.BitBucketRef{
									ID:     "refs/heads/fake-branch",
									Commit: "fake-commit-id",
								},
								ToRef: reporter.BitBucketRef{
									ID:     "refs/heads/main",
									Commit: "main-commit-id",
								},
							},
						},
					})
					assert.NoError(t, err)
					w.WriteHeader(http.StatusOK)
					_, err = w.Write(data)
					assert.NoError(t, err)
				}
				if r.URL.Path == "/rest/api/1.0/projects/proj/repos/repo/pull-requests/102/changes" {
					data, err := json.Marshal(reporter.BitBucketPullRequestChanges{
						IsLastPage: true,
						Values: []reporter.BitBucketPullRequestChange{
							{
								Path: reporter.BitBucketPath{
									ToString: "index.txt",
								},
							},
						},
					})
					assert.NoError(t, err)
					w.WriteHeader(http.StatusOK)
					_, err = w.Write(data)
					assert.NoError(t, err)
				}
				if r.URL.Path == "/rest/api/latest/projects/proj/repos/repo/pull-requests/102/activities" {
					data, err := json.Marshal(reporter.BitBucketPullRequestActivities{
						IsLastPage: true,
						Values: []reporter.BitBucketPullRequestActivity{
							{
								Action:        "COMMENTED",
								CommentAction: "ADDED",
								CommentAnchor: reporter.BitBucketCommentAnchor{
									Orphaned: false,
									DiffType: "EFFECTIVE",
									Path:     "index.txt",
									Line:     3,
								},
								Comment: reporter.BitBucketPullRequestComment{
									ID:      1001,
									Version: 0,
									State:   "OPEN",
									Author: reporter.BitBucketCommentAuthor{
										Name: "pint_user",
									},
								},
							},
							{
								Action:        "COMMENTED",
								CommentAction: "ADDED",
								CommentAnchor: reporter.BitBucketCommentAnchor{
									Orphaned: false,
									DiffType: "COMMIT",
									Path:     "index.txt",
									Line:     10,
								},
								Comment: reporter.BitBucketPullRequestComment{
									ID:      1002,
									Version: 1,
									State:   "OPEN",
									Author: reporter.BitBucketCommentAuthor{
										Name: "pint_user",
									},
								},
							},
						},
					})
					assert.NoError(t, err)
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write(data)
					return
				}
				if r.URL.Path == "/rest/api/latest/projects/proj/repos/repo/commits/fake-commit-id/diff/index.txt" {
					data, err := json.Marshal(reporter.BitBucketFileDiffs{
						Diffs: []reporter.BitBucketFileDiff{
							{
								Hunks: []reporter.BitBucketDiffHunk{
									{
										Segments: []reporter.BitBucketDiffSegment{
											{
												Type: "ADDED",
												Lines: []reporter.BitBucketDiffLine{
													{Source: 1, Destination: 1},
													{Source: 5, Destination: 5},
												},
											},
											{
												Type: "CONTEXT",
												Lines: []reporter.BitBucketDiffLine{
													{Source: 10, Destination: 6},
												},
											},
										},
									},
								},
							},
						},
					})
					assert.NoError(t, err)
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write(data)
					return
				}
				if r.URL.Path == "/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint" {
					w.WriteHeader(http.StatusOK)
					return
				}
				if r.URL.Path == "/plugins/servlet/applinks/whoami" {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("pint_user"))
					return
				}
				if r.URL.Path == "/rest/api/1.0/projects/proj/repos/repo/pull-requests/102/comments/1002" && r.Method == http.MethodDelete {
					w.WriteHeader(http.StatusOK)
					return
				}
				w.WriteHeader(http.StatusInternalServerError)
			}),
			errorHandler: func(err error) error {
				if err != nil && err.Error() == "failed to create BitBucket pull request comments: POST request failed" {
					return nil
				}
				return fmt.Errorf("Expected failed to create BitBucket pull request comments: POST request failed, got %w", err)
			},
		},
		{
			description: "sends a correct report with deduped comments",
			gitCmd:      fakeGit,
			reports: []reporter.Report{
				{
					Path: discovery.Path{
						SymlinkTarget: "foo.txt",
						Name:          "foo.txt",
					},
					ModifiedLines: []int{2, 4},
					Rule:          mockRules[1],
					Problem: checks.Problem{
						Lines: parser.LineRange{
							First: 1,
							Last:  1,
						},
						Reporter: "mock",
						Summary:  "this should be ignored, line is not part of the diff",
						Severity: checks.Bug,
					},
				},
				{
					Path: discovery.Path{
						SymlinkTarget: "foo.txt",
						Name:          "foo.txt",
					},
					ModifiedLines: []int{2, 4},
					Rule:          mockRules[1],
					Problem: checks.Problem{
						Lines: parser.LineRange{
							First: 1,
							Last:  1,
						},
						Reporter: "mock",
						Summary:  "this should be ignored, line is not part of the diff",
						Severity: checks.Bug,
					},
				},
				{
					Path: discovery.Path{
						SymlinkTarget: "foo.txt",
						Name:          "foo.txt",
					},
					ModifiedLines: []int{2, 4},
					Rule:          mockRules[1],
					Problem: checks.Problem{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "mock",
						Summary:  "bad name",
						Details:  "bad name details",
						Severity: checks.Warning,
					},
				},
				{
					Path: discovery.Path{
						SymlinkTarget: "foo.txt",
						Name:          "foo.txt",
					},
					ModifiedLines: []int{2, 4},
					Rule:          mockRules[0],
					Problem: checks.Problem{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "mock",
						Summary:  "mock text 1",
						Details:  "mock details",
						Severity: checks.Warning,
					},
				},
				{
					Path: discovery.Path{
						SymlinkTarget: "foo.txt",
						Name:          "symlink.txt",
					},
					ModifiedLines: []int{2, 4},
					Rule:          mockRules[1],
					Problem: checks.Problem{
						Lines: parser.LineRange{
							First: 2,
							Last:  2,
						},
						Reporter: "mock",
						Summary:  "mock text 2",
						Details:  "mock details",
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
					{Title: "Number of rules parsed", Type: reporter.NumberType, Value: float64(0)},
					{Title: "Number of rules checked", Type: reporter.NumberType, Value: float64(0)},
					{Title: "Number of problems found", Type: reporter.NumberType, Value: float64(5)},
					{Title: "Number of offline checks", Type: reporter.NumberType, Value: float64(0)},
					{Title: "Number of online checks", Type: reporter.NumberType, Value: float64(0)},
					{Title: "Checks duration", Type: reporter.DurationType, Value: float64(0)},
				},
			},
			pullRequestComments: []reporter.BitBucketPendingComment{
				{
					Text:     ":stop_sign: **Bug** reported by [pint](https://cloudflare.github.io/pint/) **mock** check.\n\n------\n\nthis should be ignored, line is not part of the diff\n\n------\n\n:information_source: To see documentation covering this check and instructions on how to resolve it [click here](https://cloudflare.github.io/pint/checks/mock.html).\n",
					Severity: "BLOCKER",
					Anchor: reporter.BitBucketPendingCommentAnchor{
						Path:     "foo.txt",
						Line:     1,
						LineType: "CONTEXT",
						FileType: "FROM",
						DiffType: "EFFECTIVE",
					},
				},
				{
					Text:     ":warning: **Warning** reported by [pint](https://cloudflare.github.io/pint/) **mock** check.\n\n------\n\nbad name\n\nbad name details\n\n------\n\nmock text 1\n\nmock details\n\n------\n\nmock text 2\n\nmock details\n\n:leftwards_arrow_with_hook: This problem was detected on a symlinked file `symlink.txt`.\n\n------\n\n:information_source: To see documentation covering this check and instructions on how to resolve it [click here](https://cloudflare.github.io/pint/checks/mock.html).\n",
					Severity: "NORMAL",
					Anchor: reporter.BitBucketPendingCommentAnchor{
						Path:     "foo.txt",
						Line:     2,
						LineType: "ADDED",
						FileType: "TO",
						DiffType: "EFFECTIVE",
					},
				},
			},
			pullRequests: reporter.BitBucketPullRequests{
				IsLastPage: true,
				Values: []reporter.BitBucketPullRequest{
					{
						ID:   102,
						Open: true,
						FromRef: reporter.BitBucketRef{
							ID:     "refs/heads/fake-branch",
							Commit: "fake-commit-id",
						},
						ToRef: reporter.BitBucketRef{
							ID:     "refs/heads/main",
							Commit: "main-commit-id",
						},
					},
				},
			},
			pullRequestChanges: reporter.BitBucketPullRequestChanges{
				IsLastPage: true,
				Values: []reporter.BitBucketPullRequestChange{
					{
						Path: reporter.BitBucketPath{
							ToString: "index.txt",
						},
					},
					{
						Path: reporter.BitBucketPath{
							ToString: "foo.txt",
						},
					},
				},
			},
			pullRequestFileDiffs: map[string]reporter.BitBucketFileDiffs{
				"index.txt": {
					Diffs: []reporter.BitBucketFileDiff{
						{
							Hunks: []reporter.BitBucketDiffHunk{
								{
									Segments: []reporter.BitBucketDiffSegment{
										{
											Type: "ADDED",
											Lines: []reporter.BitBucketDiffLine{
												{Source: 1, Destination: 1},
												{Source: 5, Destination: 5},
											},
										},
										{
											Type: "CONTEXT",
											Lines: []reporter.BitBucketDiffLine{
												{Source: 10, Destination: 6},
											},
										},
									},
								},
							},
						},
					},
				},
				"foo.txt": {
					Diffs: []reporter.BitBucketFileDiff{
						{
							Hunks: []reporter.BitBucketDiffHunk{
								{
									Segments: []reporter.BitBucketDiffSegment{
										{
											Type: "ADDED",
											Lines: []reporter.BitBucketDiffLine{
												{Source: 2, Destination: 2},
											},
										},
									},
								},
							},
						},
						{
							Hunks: []reporter.BitBucketDiffHunk{
								{
									Segments: []reporter.BitBucketDiffSegment{
										{
											Type: "MODIFIED",
											Lines: []reporter.BitBucketDiffLine{
												{Source: 3, Destination: 4},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			pullRequestActivities: reporter.BitBucketPullRequestActivities{
				IsLastPage: true,
				Values: []reporter.BitBucketPullRequestActivity{
					{
						Action: "APPROVED",
					},
					{
						Action:        "COMMENTED",
						CommentAction: "ADDED",
						CommentAnchor: reporter.BitBucketCommentAnchor{
							Orphaned: true,
							DiffType: "EFFECTIVE",
							Path:     "foo.txt",
							Line:     3,
						},
						Comment: reporter.BitBucketPullRequestComment{
							ID:      1001,
							Version: 0,
							State:   "OPEN",
							Author: reporter.BitBucketCommentAuthor{
								Name: "pint_user",
							},
						},
					},
					{
						Action:        "COMMENTED",
						CommentAction: "ADDED",
						CommentAnchor: reporter.BitBucketCommentAnchor{
							Orphaned: true,
							DiffType: "COMMIT",
							Path:     "foo.txt",
							Line:     10,
						},
						Comment: reporter.BitBucketPullRequestComment{
							ID:      1002,
							Version: 1,
							State:   "OPEN",
							Author: reporter.BitBucketCommentAuthor{
								Name: "pint_user",
							},
						},
					},
					{
						Action:        "COMMENTED",
						CommentAction: "ADDED",
						CommentAnchor: reporter.BitBucketCommentAnchor{
							Orphaned: false,
							DiffType: "EFFECTIVE",
							Path:     "foo.txt",
							Line:     3,
						},
						Comment: reporter.BitBucketPullRequestComment{
							ID:      2001,
							Version: 0,
							State:   "OPEN",
							Author: reporter.BitBucketCommentAuthor{
								Name: "pint_user",
							},
						},
					},
					{
						Action:        "COMMENTED",
						CommentAction: "ADDED",
						CommentAnchor: reporter.BitBucketCommentAnchor{
							Orphaned: false,
							DiffType: "COMMIT",
							Path:     "foo.txt",
							Line:     4,
						},
						Comment: reporter.BitBucketPullRequestComment{
							ID:      2002,
							Version: 1,
							State:   "OPEN",
							Author: reporter.BitBucketCommentAuthor{
								Name: "pint_user",
							},
						},
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
			slog.SetDefault(slogt.New(t))

			var commentIndex int

			var srv *httptest.Server
			if tc.httpHandler == nil {
				srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					defer r.Body.Close()

					if r.Method == http.MethodDelete {
						w.WriteHeader(http.StatusOK)
						return
					}

					if strings.HasPrefix(r.URL.Path, "/rest/api/latest/projects/proj/repos/repo/commits/fake-commit-id/diff/") {
						filename := strings.TrimPrefix(r.URL.Path, "/rest/api/latest/projects/proj/repos/repo/commits/fake-commit-id/diff/")
						assert.NotNil(t, tc.pullRequestFileDiffs)
						v, ok := tc.pullRequestFileDiffs[filename]
						assert.True(t, ok, "file is missing from pullRequestFileDiffs: %s", filename)

						data, err := json.Marshal(v)
						assert.NoError(t, err)
						w.WriteHeader(http.StatusOK)
						_, err = w.Write(data)
						assert.NoError(t, err)
						return
					}

					switch r.URL.Path {
					case "/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint":
						var resp reporter.BitBucketReport
						if err := json.NewDecoder(r.Body).Decode(&resp); err != nil {
							t.Errorf("JSON decode error: %v", err)
						}
						assert.Equal(t, tc.report, resp, "Got wrong bitbucket report body")
					case "/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint/annotations":
						var resp reporter.BitBucketAnnotations
						if err := json.NewDecoder(r.Body).Decode(&resp); err != nil {
							t.Errorf("JSON decode error: %s", err)
						}
						assert.Equal(t, tc.annotations, resp, "Got wrong bitbucket annotations")
					case "/rest/api/1.0/projects/proj/repos/repo/commits/fake-commit-id/pull-requests":
						data, err := json.Marshal(tc.pullRequests)
						assert.NoError(t, err)
						w.WriteHeader(http.StatusOK)
						_, err = w.Write(data)
						assert.NoError(t, err)
					case "/rest/api/1.0/projects/proj/repos/repo/pull-requests/102/changes":
						data, err := json.Marshal(tc.pullRequestChanges)
						assert.NoError(t, err)
						w.WriteHeader(http.StatusOK)
						_, err = w.Write(data)
						assert.NoError(t, err)
					case "/rest/api/latest/projects/proj/repos/repo/pull-requests/102/activities":
						data, err := json.Marshal(tc.pullRequestActivities)
						assert.NoError(t, err)
						w.WriteHeader(http.StatusOK)
						_, err = w.Write(data)
						assert.NoError(t, err)
					case "/rest/api/1.0/projects/proj/repos/repo/pull-requests/102/comments":
						var comment reporter.BitBucketPendingComment
						if err := json.NewDecoder(r.Body).Decode(&comment); err != nil {
							t.Errorf("JSON decode error: %s", err)
						}
						assert.Equal(t, tc.pullRequestComments[commentIndex], comment)
						commentIndex++
					case "/plugins/servlet/applinks/whoami":
						w.WriteHeader(http.StatusOK)
						_, err := w.Write([]byte("pint_user"))
						assert.NoError(t, err)
					default:
						w.WriteHeader(http.StatusInternalServerError)
						_, _ = w.Write([]byte("Unhandled path: " + r.URL.Path))
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
				50,
				tc.gitCmd)
			summary := reporter.NewSummary(tc.reports)
			err := r.Submit(summary)

			if e := tc.errorHandler(err); e != nil {
				t.Errorf("error check failure: %s", e)
				return
			}

			require.Len(t, tc.pullRequestComments, commentIndex)
		})
	}
}
