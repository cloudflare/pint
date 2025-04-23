package reporter

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v71/github"
	"golang.org/x/oauth2"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/output"
)

var reviewBody = "### This pull request was validated by [pint](https://github.com/cloudflare/pint).\n"

type GithubReporter struct {
	headCommit string

	client         *github.Client
	version        string
	baseURL        string
	uploadURL      string
	owner          string
	repo           string
	timeout        time.Duration
	prNum          int
	maxComments    int
	showDuplicates bool
}

type ghCommentMeta struct {
	id int64
}

type ghPR struct {
	files []*github.CommitFile
}

func (pr ghPR) String() string {
	return fmt.Sprintf("%d file(s)", len(pr.files))
}

func (pr ghPR) getFile(path string) *github.CommitFile {
	for _, f := range pr.files {
		if f.GetFilename() == path {
			return f
		}
	}
	return nil
}

// NewGithubReporter creates a new GitHub reporter that reports
// problems via comments on a given pull request number (integer).
func NewGithubReporter(
	ctx context.Context,
	version, baseURL, uploadURL string,
	timeout time.Duration,
	token, owner, repo string,
	prNum, maxComments int,
	headCommit string,
	showDuplicates bool,
) (_ GithubReporter, err error) {
	slog.Info(
		"Will report problems to GitHub",
		slog.String("baseURL", baseURL),
		slog.String("uploadURL", uploadURL),
		slog.String("timeout", output.HumanizeDuration(timeout)),
		slog.String("owner", owner),
		slog.String("repo", repo),
		slog.Int("pr", prNum),
		slog.Int("maxComments", maxComments),
		slog.String("headCommit", headCommit),
	)

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token}, // nolint: exhaustruct
	)
	tc := oauth2.NewClient(ctx, ts)

	gr := GithubReporter{
		client:         github.NewClient(tc),
		version:        version,
		baseURL:        baseURL,
		uploadURL:      uploadURL,
		timeout:        timeout,
		owner:          owner,
		repo:           repo,
		prNum:          prNum,
		maxComments:    maxComments,
		headCommit:     headCommit,
		showDuplicates: showDuplicates,
	}

	if gr.uploadURL != "" && gr.baseURL != "" {
		gr.client, err = gr.client.WithEnterpriseURLs(gr.baseURL, gr.uploadURL)
		if err != nil {
			return gr, fmt.Errorf("creating new GitHub client: %w", err)
		}
	}

	return gr, nil
}

func (gr GithubReporter) Describe() string {
	return "GitHub"
}

func (gr GithubReporter) Destinations(ctx context.Context) (_ []any, err error) {
	var pr ghPR
	pr.files, err = gr.listPRFiles(ctx)
	return []any{pr}, err
}

func (gr GithubReporter) Summary(ctx context.Context, _ any, s Summary, pendingComments []PendingComment, errs []error) error {
	review, err := gr.findExistingReview(ctx)
	if err != nil {
		return fmt.Errorf("failed to list pull request reviews: %w", err)
	}
	if review != nil {
		if err = gr.updateReview(ctx, review, s); err != nil {
			return fmt.Errorf("failed to update pull request review: %w", err)
		}
	} else {
		if err = gr.createReview(ctx, s); err != nil {
			return fmt.Errorf("failed to create pull request review: %w", err)
		}
	}

	if gr.maxComments > 0 && len(pendingComments) > gr.maxComments {
		if err = gr.generalComment(ctx, tooManyCommentsMsg(len(pendingComments), gr.maxComments)); err != nil {
			errs = append(errs, fmt.Errorf("failed to create general comment: %w", err))
		}
	}
	if len(errs) > 0 {
		if err = gr.generalComment(ctx, errsToComment(errs)); err != nil {
			return fmt.Errorf("failed to create general comment: %w", err)
		}
	}

	return nil
}

func (gr GithubReporter) List(ctx context.Context, _ any) ([]ExistingComment, error) {
	reqCtx, cancel := gr.reqContext(ctx)
	defer cancel()

	slog.Debug("Getting the list of pull request comments", slog.Int("pr", gr.prNum))
	existing, _, err := gr.client.PullRequests.ListComments(reqCtx, gr.owner, gr.repo, gr.prNum, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list pull request reviews: %w", err)
	}

	comments := make([]ExistingComment, 0, len(existing))
	for _, ec := range existing {
		if ec.GetPath() == "" {
			slog.Debug("Skipping general comment", slog.Int64("id", ec.GetID()))
			continue
		}
		comments = append(comments, ExistingComment{
			path: ec.GetPath(),
			text: ec.GetBody(),
			line: ec.GetLine(),
			meta: ghCommentMeta{id: ec.GetID()},
		})
	}

	return comments, nil
}

func (gr GithubReporter) Create(ctx context.Context, dst any, p PendingComment) error {
	pr := dst.(ghPR)

	file := pr.getFile(p.path)
	if file == nil {
		slog.Debug("Skipping report for path with no changes",
			slog.String("path", p.path),
		)
		return nil
	}

	diffs := parseDiffLines(file.GetPatch())
	if len(diffs) == 0 {
		slog.Debug("Skipping report for path with no diff",
			slog.String("path", p.path),
		)
		return nil
	}

	side, line := gr.fixCommentLine(dst, p)

	comment := &github.PullRequestComment{
		CommitID: github.Ptr(gr.headCommit),
		Path:     github.Ptr(p.path),
		Body:     github.Ptr(p.text),
		Line:     github.Ptr(line),
		Side:     github.Ptr(side),
	}

	slog.Debug("Creating a pr comment",
		slog.String("commit", comment.GetCommitID()),
		slog.String("path", comment.GetPath()),
		slog.Int("line", comment.GetLine()),
		slog.String("side", comment.GetSide()),
		slog.String("body", comment.GetBody()),
	)

	reqCtx, cancel := gr.reqContext(ctx)
	defer cancel()

	_, _, err := gr.client.PullRequests.CreateComment(reqCtx, gr.owner, gr.repo, gr.prNum, comment)
	return err
}

func (gr GithubReporter) Delete(_ context.Context, _ any, _ ExistingComment) error {
	return nil
}

func (gr GithubReporter) CanDelete(ExistingComment) bool {
	return false
}

func (gr GithubReporter) CanCreate(done int) bool {
	return done < gr.maxComments
}

func (gr GithubReporter) IsEqual(dst any, existing ExistingComment, pending PendingComment) bool {
	if existing.path != pending.path {
		return false
	}
	_, line := gr.fixCommentLine(dst, pending)
	if existing.line != line {
		return false
	}
	return strings.Trim(existing.text, "\n") == strings.Trim(pending.text, "\n")
}

func (gr GithubReporter) findExistingReview(ctx context.Context) (*github.PullRequestReview, error) {
	reqCtx, cancel := gr.reqContext(ctx)
	defer cancel()

	reviews, _, err := gr.client.PullRequests.ListReviews(reqCtx, gr.owner, gr.repo, gr.prNum, nil)
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

func (gr GithubReporter) updateReview(ctx context.Context, review *github.PullRequestReview, summary Summary) error {
	slog.Info("Updating pull request review", slog.String("repo", fmt.Sprintf("%s/%s", gr.owner, gr.repo)))

	reqCtx, cancel := gr.reqContext(ctx)
	defer cancel()

	_, _, err := gr.client.PullRequests.UpdateReview(
		reqCtx,
		gr.owner,
		gr.repo,
		gr.prNum,
		review.GetID(),
		formatGHReviewBody(gr.version, summary, gr.showDuplicates),
	)
	return err
}

func (gr GithubReporter) createReview(ctx context.Context, summary Summary) error {
	slog.Info("Creating pull request review", slog.String("repo", fmt.Sprintf("%s/%s", gr.owner, gr.repo)), slog.String("commit", gr.headCommit))

	reqCtx, cancel := gr.reqContext(ctx)
	defer cancel()

	review := github.PullRequestReviewRequest{
		CommitID: github.Ptr(gr.headCommit),
		Body:     github.Ptr(formatGHReviewBody(gr.version, summary, gr.showDuplicates)),
		Event:    github.Ptr("COMMENT"),
	}
	slog.Debug("Creating a review",
		slog.String("commit", review.GetCommitID()),
		slog.String("body", review.GetBody()),
	)
	_, resp, err := gr.client.PullRequests.CreateReview(
		reqCtx,
		gr.owner,
		gr.repo,
		gr.prNum,
		&review,
	)
	if err != nil {
		return err
	}
	slog.Info("Pull request review created", slog.String("status", resp.Status))
	return nil
}

func (gr GithubReporter) listPRFiles(ctx context.Context) ([]*github.CommitFile, error) {
	reqCtx, cancel := gr.reqContext(ctx)
	defer cancel()

	slog.Debug("Getting the list of modified files", slog.Int("pr", gr.prNum))
	files, _, err := gr.client.PullRequests.ListFiles(reqCtx, gr.owner, gr.repo, gr.prNum, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list pull request files: %w", err)
	}
	return files, nil
}

func formatGHReviewBody(version string, summary Summary, showDuplicates bool) string {
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

	b.WriteString("| Number of rules parsed | ")
	b.WriteString(strconv.Itoa(summary.TotalEntries))
	b.WriteString(" |\n")

	b.WriteString("| Number of rules checked | ")
	b.WriteString(strconv.FormatInt(summary.CheckedEntries, 10))
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
	if len(summary.Reports()) > 0 {
		buf := bytes.NewBuffer(nil)
		cr := NewConsoleReporter(buf, checks.Information, true, showDuplicates)
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

	if details := makePrometheusDetailsComment(summary); details != "" {
		b.WriteString(details)
	}

	return b.String()
}

func (gr GithubReporter) generalComment(ctx context.Context, body string) error {
	comment := github.IssueComment{
		Body: github.Ptr(body),
	}

	slog.Debug("Creating PR comment", slog.String("body", comment.GetBody()))

	reqCtx, cancel := gr.reqContext(ctx)
	defer cancel()

	_, _, err := gr.client.Issues.CreateComment(reqCtx, gr.owner, gr.repo, gr.prNum, &comment)
	return err
}

func (gr GithubReporter) reqContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithValue(ctx, github.SleepUntilPrimaryRateLimitResetWhenRateLimited, true), gr.timeout)
}

func (gr GithubReporter) fixCommentLine(dst any, p PendingComment) (string, int) {
	pr := dst.(ghPR)
	file := pr.getFile(p.path)

	var side string
	if p.anchor == checks.AnchorBefore {
		side = "LEFT"
	} else {
		side = "RIGHT"
	}

	line := p.line
	diffs := parseDiffLines(file.GetPatch())
	dl, ok := diffLineFor(diffs, p.line)
	switch {
	case ok && dl.wasModified && p.anchor == checks.AnchorAfter:
		// Comment on new or modified line.
		line = dl.new
	case ok && dl.wasModified && p.anchor == checks.AnchorBefore:
		// Comment on new or modified line.
		line = dl.old
	default:
		// Comment on unmodified line.
		// Find first modified line and put it there.
		for _, d := range diffs {
			if !d.wasModified {
				continue
			}
			line = d.new
			side = "RIGHT"
			break
		}
	}

	return side, line
}
