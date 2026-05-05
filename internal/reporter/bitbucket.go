package reporter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/git"
	"github.com/cloudflare/pint/internal/output"
)

type BitBucketRef struct {
	ID     string `json:"id"`
	Commit string `json:"latestCommit"`
}

type BitBucketPullRequest struct {
	FromRef BitBucketRef `json:"fromRef"`
	ToRef   BitBucketRef `json:"toRef"`
	ID      int          `json:"id"`
	Open    bool         `json:"open"`
}

type BitBucketPullRequests struct {
	Values        []BitBucketPullRequest `json:"values"`
	Start         int                    `json:"start"`
	NextPageStart int                    `json:"nextPageStart"`
	IsLastPage    bool                   `json:"isLastPage"`
}

type bitBucketPR struct {
	srcBranch string
	srcHead   string
	dstBranch string
	dstHead   string
	ID        int
}

type bitBucketCommentMeta struct {
	id      int
	version int
}

type bitBucketComment struct {
	text    string
	anchor  BitBucketCommentAnchor
	id      int
	version int
}

type BitBucketCommentAuthor struct {
	Name string `json:"name"`
}

type BitBucketPullRequestComment struct {
	State    string                        `json:"state"`
	Author   BitBucketCommentAuthor        `json:"author"`
	Text     string                        `json:"text"`
	Severity string                        `json:"severity"`
	Comments []BitBucketPullRequestComment `json:"comments"`
	ID       int                           `json:"id"`
	Version  int                           `json:"version"`
	Resolved bool                          `json:"threadResolved"`
}

type BitBucketCommentAnchor struct {
	LineType string `json:"lineType"`
	DiffType string `json:"diffType"`
	Path     string `json:"path"`
	Line     int    `json:"line"`
	Orphaned bool   `json:"orphaned"`
}

type BitBucketPullRequestActivity struct {
	Action        string                      `json:"action"`
	CommentAction string                      `json:"commentAction"`
	CommentAnchor BitBucketCommentAnchor      `json:"commentAnchor"`
	Comment       BitBucketPullRequestComment `json:"comment"`
}

type BitBucketPullRequestActivities struct {
	Values        []BitBucketPullRequestActivity `json:"values"`
	Start         int                            `json:"start"`
	NextPageStart int                            `json:"nextPageStart"`
	IsLastPage    bool                           `json:"isLastPage"`
}

type bitBucketPendingCommentAnchor struct {
	Path     string `json:"path,omitempty"`
	LineType string `json:"lineType,omitempty"`
	FileType string `json:"fileType,omitempty"`
	DiffType string `json:"diffType"`
	Line     int    `json:"line,omitempty"`
}

type BitBucketPendingComment struct {
	Text     string                        `json:"text"`
	Severity string                        `json:"severity"`
	Anchor   bitBucketPendingCommentAnchor `json:"anchor"`
}

func newBitBucketAPI(uri string, timeout time.Duration, token, project, repo string) *bitBucketAPI {
	return &bitBucketAPI{
		uri:       uri,
		timeout:   timeout,
		authToken: token,
		project:   project,
		repo:      repo,
	}
}

type bitBucketAPI struct {
	uri       string
	authToken string
	project   string
	repo      string
	timeout   time.Duration
}

func (bb bitBucketAPI) request(method, path string, body io.Reader) ([]byte, error) {
	slog.LogAttrs(context.Background(), slog.LevelInfo, "Sending a request to BitBucket", slog.String("method", method), slog.String("path", path))

	if body != nil {
		payload, _ := io.ReadAll(body)
		slog.LogAttrs(context.Background(), slog.LevelDebug, "Request payload", slog.String("body", string(payload)))
		body = bytes.NewReader(payload)
	}

	req, err := http.NewRequest(method, bb.uri+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+bb.authToken)

	netClient := &http.Client{
		Timeout: bb.timeout,
	}

	resp, err := netClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return data, err
	}

	slog.LogAttrs(context.Background(), slog.LevelInfo, "BitBucket request completed", slog.Int("status", resp.StatusCode))
	slog.LogAttrs(context.Background(), slog.LevelDebug, "BitBucket response body", slog.Int("code", resp.StatusCode), slog.String("body", string(data)))
	if resp.StatusCode >= 300 {
		slog.LogAttrs(context.Background(), slog.LevelError,
			"Got a non 2xx response",
			slog.String("body", string(data)),
			slog.String("path", path),
			slog.Int("code", resp.StatusCode),
		)
		return data, fmt.Errorf("%s request failed", method)
	}

	return data, err
}

func (bb bitBucketAPI) whoami() (string, error) {
	resp, err := bb.request(http.MethodGet, "/plugins/servlet/applinks/whoami", nil)
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(string(resp), "\n"), nil
}

func (bb bitBucketAPI) findPullRequestForBranch(branch, commit string) (*bitBucketPR, error) {
	var start int
	for {
		resp, err := bb.request(
			http.MethodGet,
			fmt.Sprintf("/rest/api/1.0/projects/%s/repos/%s/commits/%s/pull-requests?start=%d", bb.project, bb.repo, commit, start),
			nil,
		)
		if err != nil {
			return nil, err
		}

		var prs BitBucketPullRequests
		if err = json.Unmarshal(resp, &prs); err != nil {
			return nil, err
		}

		for _, pr := range prs.Values {
			if !pr.Open {
				continue
			}
			srcBranch := strings.TrimPrefix(pr.FromRef.ID, "refs/heads/")
			dstBranch := strings.TrimPrefix(pr.ToRef.ID, "refs/heads/")
			if srcBranch == branch {
				return &bitBucketPR{
					ID:        pr.ID,
					srcBranch: srcBranch,
					srcHead:   pr.FromRef.Commit,
					dstBranch: dstBranch,
					dstHead:   pr.ToRef.Commit,
				}, nil
			}
		}

		if prs.IsLastPage || prs.NextPageStart == start {
			break
		}
		start = prs.NextPageStart
	}

	return nil, nil
}

func (bb bitBucketAPI) getPullRequestComments(pr *bitBucketPR) ([]bitBucketComment, error) {
	username, err := bb.whoami()
	if err != nil {
		return nil, err
	}

	comments := []bitBucketComment{}

	var start int
	for {
		resp, err := bb.request(
			http.MethodGet,
			fmt.Sprintf(
				"/rest/api/latest/projects/%s/repos/%s/pull-requests/%d/activities?start=%d",
				bb.project, bb.repo,
				pr.ID,
				start,
			),
			nil,
		)
		if err != nil {
			return nil, err
		}

		var acts BitBucketPullRequestActivities
		if err = json.Unmarshal(resp, &acts); err != nil {
			return nil, err
		}

		for _, act := range acts.Values {
			if act.Action != "COMMENTED" {
				continue
			}
			if act.CommentAction != "ADDED" {
				continue
			}
			if act.Comment.State != "OPEN" {
				continue
			}
			if act.Comment.Author.Name != username {
				continue
			}
			if act.Comment.Severity == "BLOCKER" && act.Comment.Resolved {
				continue
			}
			if act.Comment.Severity == "NORMAL" && act.CommentAnchor.Orphaned {
				continue
			}
			comments = append(comments, bitBucketComment{
				id:      act.Comment.ID,
				version: act.Comment.Version,
				text:    act.Comment.Text,
				anchor:  act.CommentAnchor,
			})
		}

		if acts.IsLastPage || acts.NextPageStart == start {
			break
		}
		start = acts.NextPageStart
	}

	return comments, nil
}

func (bb bitBucketAPI) createComment(pr *bitBucketPR, comment BitBucketPendingComment) error {
	payload, _ := json.Marshal(comment)
	_, err := bb.request(
		http.MethodPost,
		fmt.Sprintf(
			"/rest/api/1.0/projects/%s/repos/%s/pull-requests/%d/comments",
			bb.project, bb.repo, pr.ID,
		),
		bytes.NewReader(payload),
	)
	return err
}

func (bb bitBucketAPI) deleteComment(pr *bitBucketPR, commentID, version int) error {
	_, err := bb.request(
		http.MethodDelete,
		fmt.Sprintf(
			"/rest/api/1.0/projects/%s/repos/%s/pull-requests/%d/comments/%d?version=%d",
			bb.project, bb.repo, pr.ID, commentID, version,
		),
		nil,
	)
	return err
}

func NewBitBucketReporter(uri string, timeout time.Duration, token, project, repo string, maxComments int, gitCmd git.CommandRunner) BitBucketReporter {
	slog.LogAttrs(context.Background(), slog.LevelInfo,
		"Will report problems to BitBucket",
		slog.String("uri", uri),
		slog.String("timeout", output.HumanizeDuration(timeout)),
		slog.String("project", project),
		slog.String("repo", repo),
		slog.Int("maxComments", maxComments),
	)
	return BitBucketReporter{
		api:         newBitBucketAPI(uri, timeout, token, project, repo),
		gitCmd:      gitCmd,
		maxComments: maxComments,
	}
}

type BitBucketReporter struct {
	api         *bitBucketAPI
	gitCmd      git.CommandRunner
	maxComments int
}

func (bb BitBucketReporter) Describe() string {
	return "BitBucket"
}

func (bb BitBucketReporter) Destinations(ctx context.Context) ([]any, error) {
	headCommit, err := git.HeadCommit(bb.gitCmd)
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD commit: %w", err)
	}
	slog.LogAttrs(ctx, slog.LevelInfo, "Got HEAD commit from git", slog.String("commit", headCommit))

	headBranch, err := git.CurrentBranch(bb.gitCmd)
	if err != nil {
		return nil, fmt.Errorf("failed to get current branch: %w", err)
	}

	pr, err := bb.api.findPullRequestForBranch(headBranch, headCommit)
	if err != nil {
		return nil, fmt.Errorf("failed to get open pull requests from BitBucket: %w", err)
	}

	if pr == nil {
		slog.LogAttrs(ctx, slog.LevelInfo,
			"No open pull request found",
			slog.String("branch", headBranch),
			slog.String("commit", headCommit),
		)
		return nil, nil
	}

	slog.LogAttrs(ctx, slog.LevelInfo,
		"Found open pull request, reporting problems using comments",
		slog.Int("id", pr.ID),
		slog.String("srcBranch", pr.srcBranch),
		slog.String("srcCommit", pr.srcHead),
		slog.String("dstBranch", pr.dstBranch),
		slog.String("dstCommit", pr.dstHead),
	)

	return []any{pr}, nil
}

func (bb BitBucketReporter) Summary(_ context.Context, _ any, _ Summary, _ []PendingComment, _ []error) error {
	return nil
}

func (bb BitBucketReporter) List(ctx context.Context, dst any) ([]ExistingComment, error) {
	pr := dst.(*bitBucketPR)
	comments, err := bb.api.getPullRequestComments(pr)
	if err != nil {
		return nil, err
	}
	slog.LogAttrs(ctx, slog.LevelInfo, "Got existing pull request comments from BitBucket", slog.Int("count", len(comments)))

	existing := make([]ExistingComment, 0, len(comments))
	for _, c := range comments {
		existing = append(existing, ExistingComment{
			path: c.anchor.Path,
			line: c.anchor.Line,
			text: c.text,
			meta: bitBucketCommentMeta{
				id:      c.id,
				version: c.version,
			},
		})
	}
	return existing, nil
}

func (bb BitBucketReporter) Create(ctx context.Context, dst any, p PendingComment) error {
	pr := dst.(*bitBucketPR)

	anchor := bitBucketPendingCommentAnchor{
		Path:     p.path,
		Line:     p.line,
		DiffType: "EFFECTIVE",
		LineType: "CONTEXT",
		FileType: "FROM",
	}

	if p.anchor == checks.AnchorBefore {
		anchor.LineType = "REMOVED"
	} else if p.changedLines.HasAfter(p.line) {
		anchor.LineType = "ADDED"
		anchor.FileType = "TO"
	}

	if anchor.FileType == "FROM" && p.anchor != checks.AnchorBefore {
		if before := p.changedLines.BeforeForAfter(p.line); before != p.line {
			anchor.Line = before
		}
	}

	var severity string
	if strings.HasPrefix(p.text, ":stop_sign:") {
		severity = "BLOCKER"
	} else {
		severity = "NORMAL"
	}

	slog.LogAttrs(ctx, slog.LevelDebug, "Creating BitBucket comment",
		slog.String("path", anchor.Path),
		slog.Int("line", anchor.Line),
		slog.String("lineType", anchor.LineType),
		slog.String("fileType", anchor.FileType),
		slog.String("severity", severity),
	)

	return bb.api.createComment(pr, BitBucketPendingComment{
		Text:     p.text,
		Severity: severity,
		Anchor:   anchor,
	})
}

func (bb BitBucketReporter) Delete(ctx context.Context, dst any, e ExistingComment) error {
	pr := dst.(*bitBucketPR)
	meta := e.meta.(bitBucketCommentMeta)
	slog.LogAttrs(ctx, slog.LevelDebug, "Deleting BitBucket comment",
		slog.Int("id", meta.id),
		slog.String("path", e.path),
		slog.Int("line", e.line),
	)
	return bb.api.deleteComment(pr, meta.id, meta.version)
}

func (bb BitBucketReporter) CanCreate(done int) bool {
	return done < bb.maxComments
}

func (bb BitBucketReporter) CanDelete(ExistingComment) bool {
	return true
}

func (bb BitBucketReporter) IsEqual(_ any, existing ExistingComment, pending PendingComment) bool {
	if existing.path != pending.path {
		return false
	}
	if existing.line != pending.line {
		return false
	}
	return strings.Trim(existing.text, "\n") == strings.Trim(pending.text, "\n")
}
