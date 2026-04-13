package reporter_test

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/neilotoole/slogt"
	"go.nhat.io/httpmock"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/git"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/reporter"
)

func TestBitBucketReporter(t *testing.T) {
	type errorCheck func(err error) error

	type testCaseT struct {
		mock           httpmock.Mocker
		gitCmd         git.CommandRunner
		errorHandler   errorCheck
		description    string
		reports        []reporter.Report
		showDuplicates bool
	}

	p := parser.NewParser(parser.DefaultOptions)
	mockFile := p.Parse(strings.NewReader(`
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

	diagFile := filepath.Join(t.TempDir(), "diag.txt")
	if err := os.WriteFile(diagFile, []byte("- record: target is down\n  expr: up == 0\n"), 0o644); err != nil {
		t.Fatal(err)
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
			mock: httpmock.New(func(_ *httpmock.Server) {}),
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
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectDelete("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint").
					Once()
				s.ExpectPut("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint").
					Once()
			}),
			errorHandler: func(err error) error {
				if err != nil && err.Error() == "failed to get current branch: git branch error" {
					return nil
				}
				return fmt.Errorf("Expected git branch error, got %w", err)
			},
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
					Rule:          mockFile.Groups[0].Rules[0],
					Problem:       checks.Problem{},
				},
			},
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectDelete("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint").
					Once()
				s.ExpectPut("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint").
					ReturnCode(http.StatusBadRequest).
					Return("Bad Request").
					Once()
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
			reports: []reporter.Report{
				{
					Path: discovery.Path{
						SymlinkTarget: "foo.txt",
						Name:          "foo.txt",
					},
					ModifiedLines: []int{2},
					Rule:          mockFile.Groups[0].Rules[0],
					Problem:       checks.Problem{},
				},
			},
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectDelete("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint").
					Once()
				s.ExpectPut("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint").
					Run(func(_ *http.Request) ([]byte, error) {
						time.Sleep(time.Second * 2)
						return []byte("Bad Request"), nil
					}).
					ReturnCode(http.StatusBadRequest).
					Once()
			}),
			errorHandler: func(err error) error {
				if neterr, ok := errors.AsType[net.Error](errors.Unwrap(err)); ok && neterr.Timeout() {
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
					Rule:          mockFile.Groups[0].Rules[0],
					Problem:       checks.Problem{},
				},
			},
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectDelete("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint").
					Once()
				s.ExpectPut("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint").
					Run(func(_ *http.Request) ([]byte, error) {
						time.Sleep(time.Second * 2)
						return []byte("Bad Request"), nil
					}).
					ReturnCode(http.StatusBadRequest).
					Once()
			}),
			errorHandler: func(err error) error {
				if neterr, ok := errors.AsType[net.Error](errors.Unwrap(err)); ok && neterr.Timeout() {
					return nil
				}
				return fmt.Errorf("Expected a timeout error, got %w", err)
			},
		},
		{
			description: "sends a correct report that fails",
			gitCmd:      fakeGit,
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectDelete("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint").
					Once()
				s.ExpectPut("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint").
					ReturnCode(http.StatusInternalServerError).
					Return("Internal error").
					Once()
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
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectDelete("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint").
					Once()
				s.ExpectPut("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint").
					Once()
				s.ExpectGet("/rest/api/1.0/projects/proj/repos/repo/commits/fake-commit-id/pull-requests?start=0").
					ReturnJSON(reporter.BitBucketPullRequests{
						IsLastPage: true,
						Values:     []reporter.BitBucketPullRequest{},
					}).
					Once()
				s.ExpectDelete("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint/annotations").
					ReturnCode(http.StatusInternalServerError).
					Return("Internal error").
					Once()
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
					Rule:          mockFile.Groups[0].Rules[1],
					Problem: checks.Problem{
						Lines: diags.LineRange{
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
					Rule:          mockFile.Groups[0].Rules[1],
					Problem: checks.Problem{
						Lines: diags.LineRange{
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
					Rule:          mockFile.Groups[0].Rules[1],
					Problem: checks.Problem{
						Lines: diags.LineRange{
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
					Rule:          mockFile.Groups[0].Rules[0],
					Problem: checks.Problem{
						Lines: diags.LineRange{
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
					Rule:          mockFile.Groups[0].Rules[1],
					Problem: checks.Problem{
						Lines: diags.LineRange{
							First: 4,
							Last:  4,
						},
						Reporter: "mock",
						Summary:  "mock text 2",
						Severity: checks.Warning,
					},
				},
			},
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectDelete("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint").
					Once()
				s.ExpectPut("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint").
					Once()
				s.ExpectGet("/rest/api/1.0/projects/proj/repos/repo/commits/fake-commit-id/pull-requests?start=0").
					ReturnJSON(reporter.BitBucketPullRequests{
						IsLastPage: true,
						Values:     []reporter.BitBucketPullRequest{},
					}).
					Once()
				s.ExpectDelete("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint/annotations").
					Once()
				s.ExpectPost("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint/annotations").
					ReturnCode(http.StatusInternalServerError).
					Return("Internal error").
					Once()
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
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectDelete("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint").
					Once()
				s.ExpectPut("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint").
					Once()
				s.ExpectGet("/rest/api/1.0/projects/proj/repos/repo/commits/fake-commit-id/pull-requests?start=0").
					ReturnCode(http.StatusInternalServerError).
					Return("Internal error").
					Once()
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
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectDelete("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint").
					Once()
				s.ExpectPut("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint").
					Once()
				s.ExpectGet("/rest/api/1.0/projects/proj/repos/repo/commits/fake-commit-id/pull-requests?start=0").
					ReturnJSON(reporter.BitBucketPullRequests{
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
					}).
					Once()
				s.ExpectGet("/rest/api/1.0/projects/proj/repos/repo/pull-requests/102/changes?start=0").
					ReturnCode(http.StatusInternalServerError).
					Return("Internal error").
					Once()
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
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectDelete("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint").
					Once()
				s.ExpectPut("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint").
					Once()
				s.ExpectGet("/rest/api/1.0/projects/proj/repos/repo/commits/fake-commit-id/pull-requests?start=0").
					ReturnJSON(reporter.BitBucketPullRequests{
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
					}).
					Once()
				s.ExpectGet("/rest/api/1.0/projects/proj/repos/repo/pull-requests/102/changes?start=0").
					ReturnJSON(reporter.BitBucketPullRequestChanges{
						IsLastPage: true,
						Values:     []reporter.BitBucketPullRequestChange{},
					}).
					Once()
				s.ExpectGet("/plugins/servlet/applinks/whoami").
					Return("testuser").
					Once()
				s.ExpectGet("/rest/api/latest/projects/proj/repos/repo/pull-requests/102/activities?start=0").
					ReturnCode(http.StatusInternalServerError).
					Return("Internal error").
					Once()
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
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectDelete("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint").
					Once()
				s.ExpectPut("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint").
					Once()
				s.ExpectGet("/rest/api/1.0/projects/proj/repos/repo/commits/fake-commit-id/pull-requests?start=0").
					ReturnJSON(reporter.BitBucketPullRequests{IsLastPage: true}).
					Once()
				s.ExpectDelete("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint/annotations").
					Once()
				s.ExpectPost("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint/annotations").
					Once()
			}),
			reports: []reporter.Report{
				{
					Path: discovery.Path{
						SymlinkTarget: "foo.txt",
						Name:          "foo.txt",
					},
					ModifiedLines: []int{2, 4},
					Rule:          mockFile.Groups[0].Rules[1],
					Problem: checks.Problem{
						Lines: diags.LineRange{
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
					Rule:          mockFile.Groups[0].Rules[1],
					Problem: checks.Problem{
						Lines: diags.LineRange{
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
					Rule:          mockFile.Groups[0].Rules[0],
					Problem: checks.Problem{
						Lines: diags.LineRange{
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
					Rule:          mockFile.Groups[0].Rules[1],
					Problem: checks.Problem{
						Lines: diags.LineRange{
							First: 4,
							Last:  4,
						},
						Reporter: "mock",
						Summary:  "mock text 2",
						Severity: checks.Warning,
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
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectDelete("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint").
					Once()
				s.ExpectPut("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint").
					Once()
				s.ExpectGet("/rest/api/1.0/projects/proj/repos/repo/commits/fake-commit-id/pull-requests?start=0").
					ReturnJSON(reporter.BitBucketPullRequests{IsLastPage: true}).
					Once()
				s.ExpectDelete("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint/annotations").
					Once()
				s.ExpectPost("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint/annotations").
					Once()
			}),
			reports: []reporter.Report{
				{
					Path: discovery.Path{
						SymlinkTarget: "foo.txt",
						Name:          "foo.txt",
					},
					ModifiedLines: []int{3, 4},
					Rule:          mockFile.Groups[0].Rules[1],
					Problem: checks.Problem{
						Lines: diags.LineRange{
							First: 1,
							Last:  1,
						},
						Reporter: "test/mock",
						Summary:  "syntax error",
						Severity: checks.Fatal,
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
			// Covers bitbucket.go:49-51 — deleteReport error is logged but does not stop the flow.
			description: "deleteReport fails but flow continues",
			gitCmd:      fakeGit,
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectDelete("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint").
					ReturnCode(http.StatusInternalServerError).
					Once()
				s.ExpectPut("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint").
					Once()
				s.ExpectGet("/rest/api/1.0/projects/proj/repos/repo/commits/fake-commit-id/pull-requests?start=0").
					ReturnJSON(reporter.BitBucketPullRequests{IsLastPage: true}).
					Once()
				s.ExpectDelete("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint/annotations").
					Once()
			}),
			errorHandler: func(err error) error {
				if err != nil {
					return fmt.Errorf("Unpexpected error: %w", err)
				}
				return nil
			},
		},
		{
			description: "sends a correct empty report",
			gitCmd:      fakeGit,
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectDelete("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint").
					Once()
				s.ExpectPut("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint").
					Once()
				s.ExpectGet("/rest/api/1.0/projects/proj/repos/repo/commits/fake-commit-id/pull-requests?start=0").
					ReturnJSON(reporter.BitBucketPullRequests{IsLastPage: true}).
					Once()
				s.ExpectDelete("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint/annotations").
					Once()
			}),
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
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectDelete("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint").
					Once()
				s.ExpectPut("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint").
					Once()
				s.ExpectGet("/rest/api/1.0/projects/proj/repos/repo/commits/fake-commit-id/pull-requests?start=0").
					ReturnJSON(reporter.BitBucketPullRequests{IsLastPage: true}).
					Once()
				s.ExpectDelete("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint/annotations").
					Once()
				s.ExpectPost("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint/annotations").
					Once()
			}),
			reports: []reporter.Report{
				{
					Path: discovery.Path{
						SymlinkTarget: "foo.txt",
						Name:          "foo.txt",
					},
					Rule:          mockFile.Groups[0].Rules[1],
					ModifiedLines: []int{2, 4},
					Problem: checks.Problem{
						Lines: diags.LineRange{
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
					Rule:          mockFile.Groups[0].Rules[1],
					ModifiedLines: []int{2, 4},
					Problem: checks.Problem{
						Lines: diags.LineRange{
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
					Rule:          mockFile.Groups[0].Rules[0],
					ModifiedLines: []int{2, 4},
					Problem: checks.Problem{
						Lines: diags.LineRange{
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
					Rule:          mockFile.Groups[0].Rules[1],
					ModifiedLines: []int{2, 4},
					Problem: checks.Problem{
						Lines: diags.LineRange{
							First: 4,
							Last:  4,
						},
						Reporter: "mock",
						Summary:  "mock text 2",
						Severity: checks.Warning,
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
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectDelete("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint").
					Once()
				s.ExpectPut("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint").
					Once()
				s.ExpectGet("/rest/api/1.0/projects/proj/repos/repo/commits/fake-commit-id/pull-requests?start=0").
					ReturnJSON(reporter.BitBucketPullRequests{
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
					}).
					Once()
				s.ExpectGet("/rest/api/1.0/projects/proj/repos/repo/pull-requests/102/changes?start=0").
					ReturnJSON(reporter.BitBucketPullRequestChanges{IsLastPage: true}).
					Once()
				s.ExpectGet("/plugins/servlet/applinks/whoami").
					Return("pint_user").
					Once()
				s.ExpectGet("/rest/api/latest/projects/proj/repos/repo/pull-requests/102/activities?start=0").
					ReturnJSON(reporter.BitBucketPullRequestActivities{IsLastPage: true}).
					Once()
			}),
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
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectDelete("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint").
					Once()
				s.ExpectPut("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint").
					Once()
				s.ExpectGet("/rest/api/1.0/projects/proj/repos/repo/commits/fake-commit-id/pull-requests?start=0").
					ReturnJSON(reporter.BitBucketPullRequests{
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
					}).
					Once()
				s.ExpectGet("/rest/api/1.0/projects/proj/repos/repo/pull-requests/102/changes?start=0").
					ReturnJSON(reporter.BitBucketPullRequestChanges{
						IsLastPage: true,
						Values: []reporter.BitBucketPullRequestChange{
							{Path: reporter.BitBucketPath{ToString: "index.txt"}},
							{Path: reporter.BitBucketPath{ToString: "foo.txt"}},
						},
					}).
					Once()
				s.ExpectGet("/rest/api/latest/projects/proj/repos/repo/commits/fake-commit-id/diff/index.txt?contextLines=10000&since=main-commit-id&whitespace=show&withComments=false").
					ReturnJSON(reporter.BitBucketFileDiffs{
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
					}).
					Once()
				s.ExpectGet("/rest/api/latest/projects/proj/repos/repo/commits/fake-commit-id/diff/foo.txt?contextLines=10000&since=main-commit-id&whitespace=show&withComments=false").
					ReturnJSON(reporter.BitBucketFileDiffs{
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
					}).
					Once()
				s.ExpectGet("/plugins/servlet/applinks/whoami").
					Return("pint_user").
					Once()
				s.ExpectGet("/rest/api/latest/projects/proj/repos/repo/pull-requests/102/activities?start=0").
					ReturnJSON(reporter.BitBucketPullRequestActivities{
						IsLastPage: true,
						Values: []reporter.BitBucketPullRequestActivity{
							{Action: "APPROVED"},
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
									Author:  reporter.BitBucketCommentAuthor{Name: "pint_user"},
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
									Author:  reporter.BitBucketCommentAuthor{Name: "pint_user"},
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
									Author:   reporter.BitBucketCommentAuthor{Name: "pint_user"},
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
									Author:  reporter.BitBucketCommentAuthor{Name: "pint_user"},
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
									Author:  reporter.BitBucketCommentAuthor{Name: "pint_user"},
								},
							},
						},
					}).
					Once()
				// pruneComments deletes stale comments.
				s.ExpectDelete("/rest/api/1.0/projects/proj/repos/repo/pull-requests/102/comments/1001?version=0").
					Once()
				s.ExpectDelete("/rest/api/1.0/projects/proj/repos/repo/pull-requests/102/comments/1002?version=1").
					Once()
				// 1003 has 0 replies -> deleteComment.
				s.ExpectDelete("/rest/api/1.0/projects/proj/repos/repo/pull-requests/102/comments/1003?version=1").
					Once()
				s.ExpectDelete("/rest/api/1.0/projects/proj/repos/repo/pull-requests/102/comments/2001?version=0").
					Once()
				s.ExpectDelete("/rest/api/1.0/projects/proj/repos/repo/pull-requests/102/comments/2002?version=1").
					Once()
				// addComments posts 4 new comments.
				s.ExpectPost("/rest/api/1.0/projects/proj/repos/repo/pull-requests/102/comments").
					Once()
				s.ExpectPost("/rest/api/1.0/projects/proj/repos/repo/pull-requests/102/comments").
					Once()
				s.ExpectPost("/rest/api/1.0/projects/proj/repos/repo/pull-requests/102/comments").
					Once()
				s.ExpectPost("/rest/api/1.0/projects/proj/repos/repo/pull-requests/102/comments").
					Once()
			}),
			reports: []reporter.Report{
				{
					Path: discovery.Path{
						SymlinkTarget: "foo.txt",
						Name:          "foo.txt",
					},
					ModifiedLines: []int{2, 4},
					Rule:          mockFile.Groups[0].Rules[1],
					Problem: checks.Problem{
						Lines: diags.LineRange{
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
					Rule:          mockFile.Groups[0].Rules[1],
					Problem: checks.Problem{
						Lines: diags.LineRange{
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
					Rule:          mockFile.Groups[0].Rules[0],
					Problem: checks.Problem{
						Lines: diags.LineRange{
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
					Rule:          mockFile.Groups[0].Rules[1],
					Problem: checks.Problem{
						Lines: diags.LineRange{
							First: 4,
							Last:  4,
						},
						Reporter: "mock",
						Summary:  "mock text 2",
						Severity: checks.Warning,
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
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectDelete("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint").
					Once()
				s.ExpectPut("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint").
					Once()
				s.ExpectGet("/rest/api/1.0/projects/proj/repos/repo/commits/fake-commit-id/pull-requests?start=0").
					ReturnJSON(reporter.BitBucketPullRequests{
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
					}).
					Once()
				s.ExpectGet("/rest/api/1.0/projects/proj/repos/repo/pull-requests/102/changes?start=0").
					ReturnJSON(reporter.BitBucketPullRequestChanges{
						IsLastPage: true,
						Values: []reporter.BitBucketPullRequestChange{
							{
								Path: reporter.BitBucketPath{
									ToString: "index.txt",
								},
							},
						},
					}).
					Once()
				s.ExpectGet("/rest/api/latest/projects/proj/repos/repo/commits/fake-commit-id/diff/index.txt?contextLines=10000&since=main-commit-id&whitespace=show&withComments=false").
					ReturnJSON(reporter.BitBucketFileDiffs{
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
					}).
					Once()
				s.ExpectGet("/plugins/servlet/applinks/whoami").
					Return("pint_user").
					Once()
				s.ExpectGet("/rest/api/latest/projects/proj/repos/repo/pull-requests/102/activities?start=0").
					ReturnJSON(reporter.BitBucketPullRequestActivities{
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
					}).
					Once()
				// pruneComments will try to delete both comments, which fails (500), but errors are only logged.
				s.ExpectDelete("/rest/api/1.0/projects/proj/repos/repo/pull-requests/102/comments/1001?version=0").
					ReturnCode(http.StatusInternalServerError).
					Once()
				s.ExpectDelete("/rest/api/1.0/projects/proj/repos/repo/pull-requests/102/comments/1002?version=1").
					ReturnCode(http.StatusInternalServerError).
					Once()
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
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectDelete("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint").
					Once()
				s.ExpectPut("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint").
					Once()
				s.ExpectGet("/rest/api/1.0/projects/proj/repos/repo/commits/fake-commit-id/pull-requests?start=0").
					ReturnJSON(reporter.BitBucketPullRequests{
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
					}).
					Once()
				s.ExpectGet("/rest/api/1.0/projects/proj/repos/repo/pull-requests/102/changes?start=0").
					ReturnJSON(reporter.BitBucketPullRequestChanges{
						IsLastPage: true,
						Values: []reporter.BitBucketPullRequestChange{
							{
								Path: reporter.BitBucketPath{
									ToString: "index.txt",
								},
							},
						},
					}).
					Once()
				s.ExpectGet("/rest/api/latest/projects/proj/repos/repo/commits/fake-commit-id/diff/index.txt?contextLines=10000&since=main-commit-id&whitespace=show&withComments=false").
					ReturnJSON(reporter.BitBucketFileDiffs{
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
					}).
					Once()
				s.ExpectGet("/plugins/servlet/applinks/whoami").
					ReturnCode(http.StatusInternalServerError).
					Once()
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
					Rule:          mockFile.Groups[0].Rules[1],
					Problem: checks.Problem{
						Lines: diags.LineRange{
							First: 1,
							Last:  1,
						},
						Reporter: "mock",
						Summary:  "this should be ignored, line is not part of the diff",
						Severity: checks.Bug,
					},
				},
			},
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectDelete("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint").
					Once()
				s.ExpectPut("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint").
					Once()
				s.ExpectGet("/rest/api/1.0/projects/proj/repos/repo/commits/fake-commit-id/pull-requests?start=0").
					ReturnJSON(reporter.BitBucketPullRequests{
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
					}).
					Once()
				s.ExpectGet("/rest/api/1.0/projects/proj/repos/repo/pull-requests/102/changes?start=0").
					ReturnJSON(reporter.BitBucketPullRequestChanges{
						IsLastPage: true,
						Values: []reporter.BitBucketPullRequestChange{
							{
								Path: reporter.BitBucketPath{
									ToString: "index.txt",
								},
							},
						},
					}).
					Once()
				s.ExpectGet("/rest/api/latest/projects/proj/repos/repo/commits/fake-commit-id/diff/index.txt?contextLines=10000&since=main-commit-id&whitespace=show&withComments=false").
					ReturnJSON(reporter.BitBucketFileDiffs{
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
					}).
					Once()
				s.ExpectGet("/plugins/servlet/applinks/whoami").
					Return("pint_user").
					Once()
				s.ExpectGet("/rest/api/latest/projects/proj/repos/repo/pull-requests/102/activities?start=0").
					ReturnJSON(reporter.BitBucketPullRequestActivities{
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
					}).
					Once()
				// pruneComments deletes both stale comments.
				s.ExpectDelete("/rest/api/1.0/projects/proj/repos/repo/pull-requests/102/comments/1001?version=0").
					Once()
				s.ExpectDelete("/rest/api/1.0/projects/proj/repos/repo/pull-requests/102/comments/1002?version=1").
					Once()
				// addComments tries to POST new comment, which fails.
				s.ExpectPost("/rest/api/1.0/projects/proj/repos/repo/pull-requests/102/comments").
					ReturnCode(http.StatusInternalServerError).
					Once()
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
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectDelete("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint").
					Once()
				s.ExpectPut("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint").
					Once()
				s.ExpectGet("/rest/api/1.0/projects/proj/repos/repo/commits/fake-commit-id/pull-requests?start=0").
					ReturnJSON(reporter.BitBucketPullRequests{
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
					}).
					Once()
				s.ExpectGet("/rest/api/1.0/projects/proj/repos/repo/pull-requests/102/changes?start=0").
					ReturnJSON(reporter.BitBucketPullRequestChanges{
						IsLastPage: true,
						Values: []reporter.BitBucketPullRequestChange{
							{Path: reporter.BitBucketPath{ToString: "index.txt"}},
							{Path: reporter.BitBucketPath{ToString: "foo.txt"}},
						},
					}).
					Once()
				s.ExpectGet("/rest/api/latest/projects/proj/repos/repo/commits/fake-commit-id/diff/index.txt?contextLines=10000&since=main-commit-id&whitespace=show&withComments=false").
					ReturnJSON(reporter.BitBucketFileDiffs{
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
					}).
					Once()
				s.ExpectGet("/rest/api/latest/projects/proj/repos/repo/commits/fake-commit-id/diff/foo.txt?contextLines=10000&since=main-commit-id&whitespace=show&withComments=false").
					ReturnJSON(reporter.BitBucketFileDiffs{
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
					}).
					Once()
				s.ExpectGet("/plugins/servlet/applinks/whoami").
					Return("pint_user").
					Once()
				s.ExpectGet("/rest/api/latest/projects/proj/repos/repo/pull-requests/102/activities?start=0").
					ReturnJSON(reporter.BitBucketPullRequestActivities{
						IsLastPage: true,
						Values: []reporter.BitBucketPullRequestActivity{
							{Action: "APPROVED"},
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
									Author:  reporter.BitBucketCommentAuthor{Name: "pint_user"},
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
									Author:  reporter.BitBucketCommentAuthor{Name: "pint_user"},
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
									Author:  reporter.BitBucketCommentAuthor{Name: "pint_user"},
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
									Author:  reporter.BitBucketCommentAuthor{Name: "pint_user"},
								},
							},
						},
					}).
					Once()
				// pruneComments deletes all stale comments.
				s.ExpectDelete("/rest/api/1.0/projects/proj/repos/repo/pull-requests/102/comments/1001?version=0").
					Once()
				s.ExpectDelete("/rest/api/1.0/projects/proj/repos/repo/pull-requests/102/comments/1002?version=1").
					Once()
				s.ExpectDelete("/rest/api/1.0/projects/proj/repos/repo/pull-requests/102/comments/2001?version=0").
					Once()
				s.ExpectDelete("/rest/api/1.0/projects/proj/repos/repo/pull-requests/102/comments/2002?version=1").
					Once()
				// addComments posts 2 deduped comments.
				s.ExpectPost("/rest/api/1.0/projects/proj/repos/repo/pull-requests/102/comments").
					Once()
				s.ExpectPost("/rest/api/1.0/projects/proj/repos/repo/pull-requests/102/comments").
					Once()
			}),
			reports: []reporter.Report{
				{
					Path: discovery.Path{
						SymlinkTarget: "foo.txt",
						Name:          "foo.txt",
					},
					ModifiedLines: []int{2, 4},
					Rule:          mockFile.Groups[0].Rules[1],
					Problem: checks.Problem{
						Lines: diags.LineRange{
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
					Rule:          mockFile.Groups[0].Rules[1],
					Problem: checks.Problem{
						Lines: diags.LineRange{
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
					Rule:          mockFile.Groups[0].Rules[1],
					Problem: checks.Problem{
						Lines: diags.LineRange{
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
					Rule:          mockFile.Groups[0].Rules[0],
					Problem: checks.Problem{
						Lines: diags.LineRange{
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
					Rule:          mockFile.Groups[0].Rules[1],
					Problem: checks.Problem{
						Lines: diags.LineRange{
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
			errorHandler: func(err error) error {
				if err != nil {
					return fmt.Errorf("Unpexpected error: %w", err)
				}
				return nil
			},
		},
		{
			description: "annotation on unmodified lines",
			gitCmd:      fakeGit,
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectDelete("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint").
					Once()
				s.ExpectPut("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint").
					Once()
				s.ExpectGet("/rest/api/1.0/projects/proj/repos/repo/commits/fake-commit-id/pull-requests?start=0").
					ReturnJSON(reporter.BitBucketPullRequests{IsLastPage: true}).
					Once()
				s.ExpectDelete("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint/annotations").
					Once()
			}),
			reports: []reporter.Report{
				{
					Path: discovery.Path{
						SymlinkTarget: "foo.txt",
						Name:          "foo.txt",
					},
					ModifiedLines: []int{},
					Rule:          mockFile.Groups[0].Rules[1],
					Problem: checks.Problem{
						Lines: diags.LineRange{
							First: 1,
							Last:  1,
						},
						Reporter: "mock",
						Summary:  "line is not part of the diff",
						Severity: checks.Bug,
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
			// Covers bitbucket_api.go:633-652 — diagnostics rendering in makeComments.
			description: "comment includes diagnostics when file is readable",
			gitCmd:      fakeGit,
			reports: []reporter.Report{
				{
					Path: discovery.Path{
						SymlinkTarget: "index.txt",
						Name:          diagFile,
					},
					ModifiedLines: []int{1},
					Rule:          mockFile.Groups[0].Rules[0],
					Problem: checks.Problem{
						Lines: diags.LineRange{
							First: 1,
							Last:  1,
						},
						Reporter: "mock",
						Summary:  "problem with diagnostics",
						Severity: checks.Bug,
						Anchor:   checks.AnchorAfter,
						Diagnostics: []diags.Diagnostic{
							{
								Message: "this is wrong",
								Pos: diags.PositionRanges{
									{Line: 1, FirstColumn: 3, LastColumn: 8},
								},
								FirstColumn: 3,
								LastColumn:  8,
							},
						},
					},
				},
			},
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectDelete("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint").
					Once()
				s.ExpectPut("/rest/insights/1.0/projects/proj/repos/repo/commits/fake-commit-id/reports/pint").
					Once()
				s.ExpectGet("/rest/api/1.0/projects/proj/repos/repo/commits/fake-commit-id/pull-requests?start=0").
					ReturnJSON(reporter.BitBucketPullRequests{
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
					}).
					Once()
				s.ExpectGet("/rest/api/1.0/projects/proj/repos/repo/pull-requests/102/changes?start=0").
					ReturnJSON(reporter.BitBucketPullRequestChanges{
						IsLastPage: true,
						Values: []reporter.BitBucketPullRequestChange{
							{
								Path: reporter.BitBucketPath{
									ToString: "index.txt",
								},
							},
						},
					}).
					Once()
				s.ExpectGet("/rest/api/latest/projects/proj/repos/repo/commits/fake-commit-id/diff/index.txt?contextLines=10000&since=main-commit-id&whitespace=show&withComments=false").
					ReturnJSON(reporter.BitBucketFileDiffs{
						Diffs: []reporter.BitBucketFileDiff{
							{
								Hunks: []reporter.BitBucketDiffHunk{
									{
										Segments: []reporter.BitBucketDiffSegment{
											{
												Type: "ADDED",
												Lines: []reporter.BitBucketDiffLine{
													{Source: 1, Destination: 1},
												},
											},
										},
									},
								},
							},
						},
					}).
					Once()
				s.ExpectGet("/plugins/servlet/applinks/whoami").
					Return("pint_user").
					Once()
				s.ExpectGet("/rest/api/latest/projects/proj/repos/repo/pull-requests/102/activities?start=0").
					ReturnJSON(reporter.BitBucketPullRequestActivities{IsLastPage: true}).
					Once()
				// addComments posts 1 new comment with diagnostics content.
				s.ExpectPost("/rest/api/1.0/projects/proj/repos/repo/pull-requests/102/comments").
					Once()
			}),
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

			srv := tc.mock(t)

			r := reporter.NewBitBucketReporter(
				"v0.0.0",
				srv.URL(),
				time.Second,
				"token",
				"proj",
				"repo",
				50,
				tc.showDuplicates,
				tc.gitCmd)
			summary := reporter.NewSummary(tc.reports)
			err := r.Submit(t.Context(), summary)

			if e := tc.errorHandler(err); e != nil {
				t.Errorf("error check failure: %s", e)
				return
			}
		})
	}
}
