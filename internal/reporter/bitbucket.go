package reporter

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/cloudflare/pint/internal/git"
	"github.com/cloudflare/pint/internal/output"
)

const (
	BitBucketDescription = "pint is a Prometheus rule linter/validator.\n" +
		"It will inspect all Prometheus recording and alerting rules for problems that could prevent these from working correctly.\n" +
		"Checks can be either offline (static checks using only rule definition) or online (validate rule against live Prometheus server)."
)

func NewBitBucketReporter(
	version, uri string,
	timeout time.Duration,
	token, project, repo string,
	maxComments int,
	gitCmd git.CommandRunner,
) BitBucketReporter {
	slog.Info(
		"Will report problems to BitBucket",
		slog.String("uri", uri),
		slog.String("timeout", output.HumanizeDuration(timeout)),
		slog.String("project", project),
		slog.String("repo", repo),
		slog.Int("maxComments", maxComments),
	)
	return BitBucketReporter{
		api:    newBitBucketAPI(version, uri, timeout, token, project, repo, maxComments),
		gitCmd: gitCmd,
	}
}

// BitBucketReporter send linter results to BitBucket using
// https://docs.atlassian.com/bitbucket-server/rest/7.8.0/bitbucket-code-insights-rest.html
type BitBucketReporter struct {
	api    *bitBucketAPI
	gitCmd git.CommandRunner
}

func (bb BitBucketReporter) Submit(summary Summary) (err error) {
	var headCommit string
	if headCommit, err = git.HeadCommit(bb.gitCmd); err != nil {
		return fmt.Errorf("failed to get HEAD commit: %w", err)
	}
	slog.Info("Got HEAD commit from git", slog.String("commit", headCommit))

	if err = bb.api.deleteReport(headCommit); err != nil {
		slog.Error("Failed to delete old BitBucket report", slog.Any("err", err))
	}

	if err = bb.api.createReport(summary, headCommit); err != nil {
		return fmt.Errorf("failed to create BitBucket report: %w", err)
	}

	var headBranch string
	if headBranch, err = git.CurrentBranch(bb.gitCmd); err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	var pr *bitBucketPR
	if pr, err = bb.api.findPullRequestForBranch(headBranch, headCommit); err != nil {
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
		if changes, err = bb.api.getPullRequestChanges(pr); err != nil {
			return fmt.Errorf("failed to get pull request changes from BitBucket: %w", err)
		}
		slog.Debug(
			"Got modified files from BitBucket",
			slog.Any("files", changes.pathModifiedLines),
		)

		var existingComments []bitBucketComment
		if existingComments, err = bb.api.getPullRequestComments(pr); err != nil {
			return fmt.Errorf("failed to get pull request comments from BitBucket: %w", err)
		}
		slog.Info(
			"Got existing pull request comments from BitBucket",
			slog.Int("count", len(existingComments)),
		)

		pendingComments := bb.api.makeComments(summary, changes)
		slog.Info("Generated comments to add to BitBucket", slog.Int("count", len(pendingComments)))

		pendingComments = bb.api.limitComments(pendingComments)
		slog.Info("Will add comments to BitBucket",
			slog.Int("count", len(pendingComments)),
			slog.Int("limit", bb.api.maxComments),
		)

		slog.Info("Deleting stale comments from BitBucket")
		bb.api.pruneComments(pr, existingComments, pendingComments)

		slog.Info("Adding missing comments to BitBucket")
		if err = bb.api.addComments(pr, existingComments, pendingComments); err != nil {
			return fmt.Errorf("failed to create BitBucket pull request comments: %w", err)
		}

	} else {
		slog.Info(
			"No open pull request found, reporting problems using code insight annotations",
			slog.String("branch", headBranch),
			slog.String("commit", headCommit),
		)

		if err = bb.api.deleteAnnotations(headCommit); err != nil {
			return fmt.Errorf("failed to delete existing BitBucket code insight annotations: %w", err)
		}

		if err = bb.api.createAnnotations(summary, headCommit); err != nil {
			return fmt.Errorf("failed to create BitBucket code insight annotations: %w", err)
		}
	}

	if summary.HasFatalProblems() {
		return errors.New("fatal error(s) reported")
	}

	return nil
}

// BitBucket only allows us to report annotations for modified lines.
// If a high severity problem is detected on a non-modified line we move that annotation
// to the first modified line.
// Without this we could have a report that is marked as failed, but with no annotations
// at all, which would make it more difficult to fix.
// If we can't find any modified line to match with our report then we return 0,
// which will create a file level annotation.
func moveReportedLine(report Report) (reported, original int) {
	reported = -1
	original = -1
	for pl := report.Problem.Lines.First; pl <= report.Problem.Lines.Last; pl++ {
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

	if reported < 0 {
		reported = 0
	}
	return reported, original
}
