package reporter

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/go-github/v37/github"
	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"

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
	ctx, cancel := context.WithTimeout(context.Background(), gr.timeout)
	defer cancel()

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: gr.authToken},
	)
	tc := oauth2.NewClient(ctx, ts)

	var client *github.Client

	if gr.uploadURL != "" && gr.baseURL != "" {
		ec, err := github.NewEnterpriseClient(gr.baseURL, gr.uploadURL, tc)
		if err != nil {
			return fmt.Errorf("creating new GitHub client: %w", err)
		}
		client = ec
	} else {
		client = github.NewClient(tc)
	}

	comments := []*github.DraftReviewComment{}
	for _, rep := range summary.Reports {
		rep := rep

		if len(rep.ModifiedLines) == 0 {
			continue
		}

		var comment *github.DraftReviewComment

		if len(rep.ModifiedLines) == 1 {
			comment = &github.DraftReviewComment{
				Path: github.String(rep.Path),
				Body: github.String(rep.Problem.Text),
				Line: github.Int(rep.ModifiedLines[0]),
			}
		} else if len(rep.ModifiedLines) > 1 {
			sort.Ints(rep.ModifiedLines)
			start, end := rep.ModifiedLines[0], rep.ModifiedLines[len(rep.ModifiedLines)-1]
			comment = &github.DraftReviewComment{
				Path:      github.String(rep.Path),
				Body:      github.String(rep.Problem.Text),
				Line:      github.Int(end),
				StartLine: github.Int(start),
			}
		}

		comments = append(comments, comment)
	}

	if len(comments) > 0 {
		_, resp, err := client.PullRequests.CreateReview(ctx, gr.owner, gr.repo, gr.prNum, &github.PullRequestReviewRequest{
			Event:    github.String("COMMENT"),
			Comments: comments,
		})
		if err != nil {
			return fmt.Errorf("creating review: %w", err)
		}
		log.Info().Str("status", resp.Status).Msg("Report submitted")
	}

	return nil
}
