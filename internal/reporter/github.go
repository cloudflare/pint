package reporter

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/go-github/v37/github"
	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/git"
)

type GithubReporter struct {
	baseURL   string
	uploadURL string
	timeout   time.Duration
	authToken string
	owner     string
	repo      string
	prNum     int
	gitCmd    git.CommandRunner
}

// NewGithubReporter creates a new GitHub reporter that reports
// problems via comments on a given pull request number (integer).
func NewGithubReporter(baseURL, uploadURL string, timeout time.Duration, token, owner, repo string, prNum int, gitCmd git.CommandRunner) GithubReporter {
	return GithubReporter{
		baseURL:   baseURL,
		uploadURL: uploadURL,
		timeout:   timeout,
		authToken: token,
		owner:     owner,
		repo:      repo,
		prNum:     prNum,
		gitCmd:    gitCmd,
	}
}

// Submit submits the summary to GitHub.
func (gr GithubReporter) Submit(summary Summary) error {
	headCommit, err := git.HeadCommit(gr.gitCmd)
	if err != nil {
		return fmt.Errorf("failed to get HEAD commit: %w", err)
	}
	log.Info().Str("commit", headCommit).Msg("Got HEAD commit from git")

	ok, err := gr.hasReview()
	fmt.Printf("ok=%v err=%s\n", ok, err)

	return gr.createReview(headCommit, summary)
}

func (gr GithubReporter) newClient() (client *github.Client, err error) {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: gr.authToken},
	)
	tc := oauth2.NewClient(context.Background(), ts)

	if gr.uploadURL != "" && gr.baseURL != "" {
		client, err = github.NewEnterpriseClient(gr.baseURL, gr.uploadURL, tc)
		if err != nil {
			return nil, fmt.Errorf("creating new GitHub client: %w", err)
		}
	} else {
		client = github.NewClient(tc)
	}

	return client, nil
}

func (gr GithubReporter) hasReview() (bool, error) {
	client, err := gr.newClient()
	if err != nil {
		return false, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), gr.timeout)
	defer cancel()

	reviews, _, err := client.PullRequests.ListReviews(ctx, gr.owner, gr.repo, gr.prNum, nil)
	if err != nil {
		return false, err
	}

	for _, review := range reviews {
		fmt.Printf("review: %+v\n", review)
	}

	return false, nil
}

func (gr GithubReporter) createReview(headCommit string, summary Summary) error {
	ctx, cancel := context.WithTimeout(context.Background(), gr.timeout)
	defer cancel()

	client, _ := gr.newClient()
	_, resp, err := client.PullRequests.CreateReview(
		ctx,
		gr.owner,
		gr.repo,
		gr.prNum,
		gr.formatReviewPayload(headCommit, summary),
	)
	if err != nil {
		return fmt.Errorf("failed to create review: %w", err)
	}
	log.Info().Str("status", resp.Status).Msg("Report submitted")
	return nil
}

func (gr GithubReporter) formatReviewPayload(headCommit string, summary Summary) *github.PullRequestReviewRequest {
	event := "APPROVE"

	comments := []*github.DraftReviewComment{}
	for _, rep := range summary.Reports() {
		rep := rep

		if rep.Problem.Severity > checks.Information {
			event = "COMMENT"
		}

		if len(rep.ModifiedLines) == 0 {
			continue
		}

		var comment *github.DraftReviewComment

		if len(rep.ModifiedLines) == 1 {
			comment = &github.DraftReviewComment{
				Path: github.String(rep.ReportedPath),
				Body: github.String(rep.Problem.Text),
				Line: github.Int(rep.ModifiedLines[0]),
			}
		} else if len(rep.ModifiedLines) > 1 {
			sort.Ints(rep.ModifiedLines)
			start, end := rep.ModifiedLines[0], rep.ModifiedLines[len(rep.ModifiedLines)-1]
			comment = &github.DraftReviewComment{
				Path:      github.String(rep.ReportedPath),
				Body:      github.String(rep.Problem.Text),
				Line:      github.Int(end),
				StartLine: github.Int(start),
			}
		}

		comments = append(comments, comment)
	}

	return &github.PullRequestReviewRequest{
		Event:    github.String(event),
		CommitID: github.String(headCommit),
		Comments: comments,
	}
}
