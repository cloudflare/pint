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
	gitCmd    git.CommandRunner
}

// NewGithubReporter creates a new GitHub reporter that reports
// problems via comments on a given pull request number (integer).
func NewGithubReporter(baseURL, uploadURL string, timeout time.Duration, token, owner, repo string, gitCmd git.CommandRunner) GithubReporter {
	return GithubReporter{
		baseURL:   baseURL,
		uploadURL: uploadURL,
		timeout:   timeout,
		authToken: token,
		owner:     owner,
		repo:      repo,
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

	ctx, cancel := context.WithTimeout(context.Background(), gr.timeout)
	defer cancel()

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: gr.authToken},
	)
	tc := oauth2.NewClient(ctx, ts)

	var client *github.Client

	if gr.uploadURL != "" && gr.baseURL != "" {
		client, err = github.NewEnterpriseClient(gr.baseURL, gr.uploadURL, tc)
		if err != nil {
			return fmt.Errorf("failed to create a new GitHub client: %w", err)
		}
	} else {
		client = github.NewClient(tc)
	}

	conclusion := "success"
	comments := []*github.CheckRunAnnotation{}
	for _, rep := range summary.Reports() {
		rep := rep

		var level string
		switch rep.Problem.Severity {
		case checks.Fatal, checks.Bug:
			level = "failure"
			conclusion = "failure"
		case checks.Warning:
			level = "warning"
		case checks.Information:
			level = "notice"
		}

		var comment *github.CheckRunAnnotation
		sort.Ints(rep.ModifiedLines)
		start, end := rep.Problem.LineRange()
		comment = &github.CheckRunAnnotation{
			Path:            github.String(rep.ReportedPath),
			StartLine:       github.Int(start),
			EndLine:         github.Int(end),
			AnnotationLevel: github.String(level),
			Message:         github.String(rep.Problem.Text),
			Title:           github.String(rep.Problem.Fragment),
		}

		comments = append(comments, comment)
	}

	_, resp, err := client.Checks.CreateCheckRun(ctx, gr.owner, gr.repo, github.CreateCheckRunOptions{
		Name:        "pint",
		HeadSHA:     headCommit,
		DetailsURL:  github.String("https://cloudflare.github.io/pint/"),
		Conclusion:  github.String(conclusion),
		CompletedAt: &github.Timestamp{Time: time.Now()},
		Output: &github.CheckRunOutput{
			Title:            github.String("pint"),
			Summary:          github.String(BitBucketDescription),
			AnnotationsCount: github.Int(len(comments)),
			Annotations:      comments,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create a new check run: %w", err)
	}
	log.Info().Str("status", resp.Status).Msg("Report submitted")

	return nil
}
