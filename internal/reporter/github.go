package reporter

import (
	"fmt"
	"time"

	"context"

	"github.com/cloudflare/pint/internal/git"
	"github.com/google/go-github/v35/github"
	"golang.org/x/oauth2"
)

type GithubReporter struct {
	baseURL   *string
	uploadURL *string
	timeout   time.Duration
	authToken string
	owner     string
	repo      string
	prNum     int
	gitCmd    git.CommandRunner
}

// NewGithubReporter creates a new GitHub reporter that reports
// problems via comments on a given pull request number (integer).
func NewGithubReporter(baseURL, uploadURL *string, timeout time.Duration, token, owner, repo string, prNum int, gitCmd git.CommandRunner) GithubReporter {
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

	if (gr.uploadURL != nil && *gr.uploadURL != "") &&
		(gr.baseURL != nil && *gr.baseURL != "") {
		ec, err := github.NewEnterpriseClient(*gr.baseURL, *gr.uploadURL, tc)
		if err != nil {
			return fmt.Errorf("creating new GitHub client: %w", err)
		}
		client = ec
	} else {
		client = github.NewClient(tc)
	}

	pb, err := blameReports(summary.Reports, gr.gitCmd)
	if err != nil {
		return fmt.Errorf("failed to run git blame: %w", err)
	}

	comments := []*github.DraftReviewComment{}
	for _, rep := range summary.Reports {
		rep := rep

		gitBlames, ok := pb[rep.Path]
		if !ok {
			continue
		}

		linesWithBlame := []int{}
		for _, pl := range rep.Problem.Lines {
			commit := gitBlames.GetCommit(pl)
			if summary.FileChanges.HasCommit(commit) {
				linesWithBlame = append(linesWithBlame, pl)
			}
		}

		if len(linesWithBlame) == 0 {
			continue
		}

		var comment *github.DraftReviewComment

		if len(linesWithBlame) == 1 {
			comment = &github.DraftReviewComment{
				Path: github.String(rep.Path),
				Body: github.String(rep.Problem.Text),
				Line: github.Int(linesWithBlame[0]),
			}
		} else if len(linesWithBlame) > 1 {
			start, end := linesWithBlame[0], linesWithBlame[len(linesWithBlame)-1]
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
		if _, _, err := client.PullRequests.CreateReview(ctx, gr.owner, gr.repo, gr.prNum, &github.PullRequestReviewRequest{
			Event:    github.String("COMMENT"),
			Comments: comments,
		}); err != nil {
			return fmt.Errorf("creating review: %w", err)
		}
	}

	return nil
}
