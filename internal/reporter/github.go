package reporter

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v55/github"
	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/git"
	"github.com/cloudflare/pint/internal/output"
)

var reviewBody = "### This pull request was validated by [pint](https://github.com/cloudflare/pint).\n"

type GithubReporter struct {
	version   string
	baseURL   string
	uploadURL string
	timeout   time.Duration
	authToken string
	owner     string
	repo      string
	prNum     int
	gitCmd    git.CommandRunner

	client *github.Client
}

// NewGithubReporter creates a new GitHub reporter that reports
// problems via comments on a given pull request number (integer).
func NewGithubReporter(version, baseURL, uploadURL string, timeout time.Duration, token, owner, repo string, prNum int, gitCmd git.CommandRunner) (_ GithubReporter, err error) {
	gr := GithubReporter{
		version:   version,
		baseURL:   baseURL,
		uploadURL: uploadURL,
		timeout:   timeout,
		authToken: token,
		owner:     owner,
		repo:      repo,
		prNum:     prNum,
		gitCmd:    gitCmd,
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: gr.authToken},
	)
	tc := oauth2.NewClient(context.Background(), ts)
	gr.client = github.NewClient(tc)

	if gr.uploadURL != "" && gr.baseURL != "" {
		gr.client, err = gr.client.WithEnterpriseURLs(gr.baseURL, gr.uploadURL)
		if err != nil {
			return gr, fmt.Errorf("creating new GitHub client: %w", err)
		}
	}

	return gr, nil
}

// Submit submits the summary to GitHub.
func (gr GithubReporter) Submit(summary Summary) error {
	headCommit, err := git.HeadCommit(gr.gitCmd)
	if err != nil {
		return fmt.Errorf("failed to get HEAD commit: %w", err)
	}
	log.Info().Str("commit", headCommit).Msg("Got HEAD commit from git")

	review, err := gr.findExistingReview()
	if err != nil {
		return fmt.Errorf("failed to list pull request reviews: %w", err)
	}
	if review != nil {
		if err = gr.updateReview(review, summary); err != nil {
			return err
		}
	} else {
		if err = gr.createReview(headCommit, summary); err != nil {
			return err
		}
	}

	return gr.addReviewComments(headCommit, summary)
}

func (gr GithubReporter) findExistingReview() (*github.PullRequestReview, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gr.timeout)
	defer cancel()

	reviews, _, err := gr.client.PullRequests.ListReviews(ctx, gr.owner, gr.repo, gr.prNum, nil)
	if err != nil {
		return nil, err
	}

	for _, review := range reviews {
		if strings.HasPrefix(review.GetBody(), reviewBody) {
			return review, nil
		}
	}

	return nil, nil
}

func (gr GithubReporter) updateReview(review *github.PullRequestReview, summary Summary) error {
	log.Info().Str("repo", fmt.Sprintf("%s/%s", gr.owner, gr.repo)).Msg("Updating pull request review")

	ctx, cancel := context.WithTimeout(context.Background(), gr.timeout)
	defer cancel()

	_, _, err := gr.client.PullRequests.UpdateReview(
		ctx,
		gr.owner,
		gr.repo,
		gr.prNum,
		review.GetID(),
		formatGHReviewBody(gr.version, summary),
	)
	return err
}

func (gr GithubReporter) addReviewComments(headCommit string, summary Summary) error {
	log.Info().Msg("Creating review comments")

	existingComments, err := gr.getReviewComments()
	if err != nil {
		return err
	}

	for _, rep := range summary.Reports() {
		comment := reportToGitHubComment(headCommit, rep)

		var found bool
		for _, ec := range existingComments {
			if ec.GetBody() == comment.GetBody() && ec.GetCommitID() == comment.GetCommitID() {
				found = true
				break
			}
		}
		if found {
			log.Debug().Str("body", comment.GetBody()).Str("commit", comment.GetCommitID()).Msg("Comment already exist")
			continue
		}

		if err := gr.createComment(comment); err != nil {
			return err
		}
	}

	return nil
}

func (gr GithubReporter) getReviewComments() ([]*github.PullRequestComment, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gr.timeout)
	defer cancel()

	comments, _, err := gr.client.PullRequests.ListComments(ctx, gr.owner, gr.repo, gr.prNum, nil)
	return comments, err
}

func (gr GithubReporter) createComment(comment *github.PullRequestComment) error {
	log.Debug().Str("body", comment.GetBody()).Str("commit", comment.GetCommitID()).Msg("Creating review comment")

	ctx, cancel := context.WithTimeout(context.Background(), gr.timeout)
	defer cancel()

	_, _, err := gr.client.PullRequests.CreateComment(ctx, gr.owner, gr.repo, gr.prNum, comment)
	return err
}

func (gr GithubReporter) createReview(headCommit string, summary Summary) error {
	log.Info().Str("repo", fmt.Sprintf("%s/%s", gr.owner, gr.repo)).Str("commit", headCommit).Msg("Creating pull request review")

	ctx, cancel := context.WithTimeout(context.Background(), gr.timeout)
	defer cancel()

	_, resp, err := gr.client.PullRequests.CreateReview(
		ctx,
		gr.owner,
		gr.repo,
		gr.prNum,
		&github.PullRequestReviewRequest{
			CommitID: github.String(headCommit),
			Body:     github.String(formatGHReviewBody(gr.version, summary)),
			Event:    github.String("COMMENT"),
		},
	)
	if err != nil {
		return fmt.Errorf("failed to create review: %w", err)
	}
	log.Info().Str("status", resp.Status).Msg("Pull request review created")
	return nil
}

func formatGHReviewBody(version string, summary Summary) string {
	var b strings.Builder

	b.WriteString(reviewBody)

	bySeverity := summary.CountBySeverity()
	if len(bySeverity) > 0 {
		b.WriteString(":heavy_exclamation_mark:	Problems found.\n")
		b.WriteString("| Severity | Number of problems |\n")
		b.WriteString("| --- | --- |\n")

		for _, s := range []checks.Severity{checks.Fatal, checks.Bug, checks.Warning, checks.Information} {
			if bySeverity[s] > 0 {
				b.WriteString("| ")
				b.WriteString(s.String())
				b.WriteString(" | ")
				b.WriteString(strconv.Itoa(bySeverity[s]))
				b.WriteString(" |\n")
			}
		}
	} else {
		b.WriteString(":heavy_check_mark: No problems found\n")
	}

	b.WriteString("<details><summary>Stats</summary>\n<p>\n\n")
	b.WriteString("| Stat | Value |\n")
	b.WriteString("| --- | --- |\n")

	b.WriteString("| Version | ")
	b.WriteString(version)
	b.WriteString(" |\n")

	b.WriteString("| Number of rules checked | ")
	b.WriteString(strconv.Itoa(summary.Entries))
	b.WriteString(" |\n")

	b.WriteString("| Number of problems found | ")
	b.WriteString(strconv.Itoa(len(summary.Reports())))
	b.WriteString(" |\n")

	b.WriteString("| Number of offline checks | ")
	b.WriteString(strconv.FormatInt(summary.OfflineChecks, 10))
	b.WriteString(" |\n")

	b.WriteString("| Number of online checks | ")
	b.WriteString(strconv.FormatInt(summary.OnlineChecks, 10))
	b.WriteString(" |\n")

	b.WriteString("| Checks duration | ")
	b.WriteString(output.HumanizeDuration(summary.Duration))
	b.WriteString(" |\n")

	b.WriteString("\n</p>\n</details>\n\n")

	b.WriteString("<details><summary>Problems</summary>\n<p>\n\n")
	if summary.Entries > 0 {
		buf := bytes.NewBuffer(nil)
		cr := NewConsoleReporter(buf, checks.Information)
		err := cr.Submit(summary)
		if err != nil {
			b.WriteString(fmt.Sprintf("Failed to generate list of problems: %s", err))
		} else {
			b.WriteString("```\n")
			b.WriteString(buf.String())
			b.WriteString("```\n")
		}
	} else {
		b.WriteString("No problems reported")
	}
	b.WriteString("\n</p>\n</details>\n\n")

	return b.String()
}

func reportToGitHubComment(headCommit string, rep Report) *github.PullRequestComment {
	var msgPrefix string
	reportLine, srcLine := moveReportedLine(rep)
	if reportLine != srcLine {
		msgPrefix = fmt.Sprintf("Problem reported on unmodified line %d, annotation moved here: ", srcLine)
	}

	c := github.PullRequestComment{
		CommitID: github.String(headCommit),
		Path:     github.String(rep.ReportedPath),
		Body: github.String(fmt.Sprintf(
			"[%s](https://cloudflare.github.io/pint/checks/%s.html): %s%s",
			rep.Problem.Reporter,
			rep.Problem.Reporter,
			msgPrefix,
			rep.Problem.Text,
		)),
		Line: github.Int(reportLine),
	}

	return &c
}
