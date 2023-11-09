package reporter

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/cloudflare/pint/internal/git"
)

const (
	BitBucketDescription = "pint is a Prometheus rule linter/validator.\n" +
		"It will inspect all Prometheus recording and alerting rules for problems that could prevent these from working correctly.\n" +
		"Checks can be either offline (static checks using only rule definition) or online (validate rule against live Prometheus server)."
)

func NewBitBucketReporter(version, uri string, timeout time.Duration, token, project, repo string, gitCmd git.CommandRunner) BitBucketReporter {
	return BitBucketReporter{
		api:    newBitBucketAPI(version, uri, timeout, token, project, repo),
		gitCmd: gitCmd,
	}
}

// BitBucketReporter send linter results to BitBucket using
// https://docs.atlassian.com/bitbucket-server/rest/7.8.0/bitbucket-code-insights-rest.html
type BitBucketReporter struct {
	api    *bitBucketAPI
	gitCmd git.CommandRunner
}

func (r BitBucketReporter) Submit(summary Summary) (err error) {
	var headCommit string
	if headCommit, err = git.HeadCommit(r.gitCmd); err != nil {
		return fmt.Errorf("failed to get HEAD commit: %w", err)
	}
	slog.Info("Got HEAD commit from git", slog.String("commit", headCommit))

	if err = r.api.deleteReport(headCommit); err != nil {
		slog.Error("Failed to delete old BitBucket report", slog.Any("err", err))
	}

	if err = r.api.createReport(summary, headCommit); err != nil {
		return fmt.Errorf("failed to create BitBucket report: %w", err)
	}

	var headBranch string
	if headBranch, err = git.CurrentBranch(r.gitCmd); err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	var pr *bitBucketPR
	if pr, err = r.api.findPullRequestForBranch(headBranch, headCommit); err != nil {
		return fmt.Errorf("failed to get open pull requests from BitBucket: %w", err)
	}

	if pr != nil {
		slog.Info(
			"Found open pull request, reporting problems using comments",
			slog.Int("id", pr.ID),
			slog.String("srcBranch", pr.srcBranch),
			slog.String("srcCommit", pr.srcHead),
			slog.String("dstBranch", pr.dstBranch),
			slog.String("dstCommit", pr.dstHead),
		)

		slog.Info("Getting pull request changes from BitBucket")
		var changes *bitBucketPRChanges
		if changes, err = r.api.getPullRequestChanges(pr); err != nil {
			return fmt.Errorf("failed to get pull request changes from BitBucket: %w", err)
		}
		slog.Info("Got modified files from BitBucket", slog.Any("files", changes.pathModifiedLines))

		var existingComments []bitBucketComment
		if existingComments, err = r.api.getPullRequestComments(pr); err != nil {
			return fmt.Errorf("failed to get pull request comments from BitBucket: %w", err)
		}
		slog.Info("Got existing pull request comments from BitBucket", slog.Int("count", len(existingComments)))

		pendingComments := r.api.makeComments(summary, changes)
		slog.Info("Generated comments to add to BitBucket", slog.Int("count", len(pendingComments)))

		slog.Info("Deleting stale comments from BitBucket")
		r.api.pruneComments(pr, existingComments, pendingComments)

		slog.Info("Adding missing comments to BitBucket")
		if err = r.api.addComments(pr, existingComments, pendingComments); err != nil {
			return fmt.Errorf("failed to create BitBucket pull request comments: %w", err)
		}

	} else {
		slog.Info(
			"No open pull request found, reporting problems using code insight annotations",
			slog.String("branch", headBranch),
			slog.String("commit", headCommit),
		)

		if err = r.api.deleteAnnotations(headCommit); err != nil {
			return fmt.Errorf("failed to delete existing BitBucket code insight annotations: %w", err)
		}

		if err = r.api.createAnnotations(summary, headCommit); err != nil {
			return fmt.Errorf("failed to create BitBucket code insight annotations: %w", err)
		}
	}

	if summary.HasFatalProblems() {
		return fmt.Errorf("fatal error(s) reported")
	}

	return nil
}

// BitBucket only allows us to report annotations for modified lines.
// If a high severity problem is detected on a non-modified line we move that annotation
// to the first modified line.
// Without this we could have a report that is marked as failed, but with no annotations
// at all, which would make it more difficult to fix.
func moveReportedLine(report Report) (reported, original int) {
	reported = -1
	original = -1
	for _, pl := range report.Problem.Lines {
		if original < 0 {
			original = pl
		}
		for _, ml := range report.ModifiedLines {
			if pl == ml {
				original = pl
				reported = pl
			}
		}
	}

	if reported < 0 && len(report.ModifiedLines) > 0 {
		return report.ModifiedLines[0], original
	}

	return reported, original
}
