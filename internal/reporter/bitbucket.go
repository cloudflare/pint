package reporter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cloudflare/pint/internal/output"
)

type BitBucketReporter struct {
	uri         string
	authToken   string
	project     string
	repo        string
	branch      string
	commit      string
	timeout     time.Duration
	maxComments int
}

func NewBitBucketReporter(
	uri string,
	timeout time.Duration,
	token, project, repo, branch, commit string,
	maxComments int,
) BitBucketReporter {
	slog.LogAttrs(
		context.Background(), slog.LevelInfo,
		"Will report problems to BitBucket",
		slog.String("uri", uri),
		slog.String("timeout", output.HumanizeDuration(timeout)),
		slog.String("project", project),
		slog.String("repo", repo),
		slog.String("branch", branch),
		slog.String("commit", commit),
		slog.Int("maxComments", maxComments),
	)
	return BitBucketReporter{
		uri:         uri,
		authToken:   token,
		project:     project,
		repo:        repo,
		branch:      branch,
		commit:      commit,
		timeout:     timeout,
		maxComments: maxComments,
	}
}

func (bb BitBucketReporter) Describe() string {
	return "BitBucket"
}

func (bb BitBucketReporter) Destinations(ctx context.Context) ([]any, error) {
	slog.LogAttrs(
		ctx, slog.LevelInfo, "Got HEAD commit from git",
		slog.String("commit", bb.commit),
		slog.String("branch", bb.branch),
	)

	pr, err := bb.findPullRequestForBranch(ctx, bb.branch, bb.commit)
	if err != nil {
		return nil, fmt.Errorf("failed to get open pull requests from BitBucket: %w", err)
	}
	if pr == nil {
		slog.LogAttrs(
			ctx, slog.LevelInfo,
			"No open pull request found, skipping BitBucket reporting",
			slog.String("branch", bb.branch),
		)
		return nil, nil
	}

	slog.LogAttrs(
		ctx, slog.LevelInfo,
		"Found open pull request",
		slog.Int("id", pr.ID),
		slog.String("srcBranch", pr.srcBranch),
		slog.String("dstBranch", pr.dstBranch),
	)
	return []any{pr}, nil
}

func (bb BitBucketReporter) Summary(
	ctx context.Context,
	dst any,
	s Summary,
	pendingComments []PendingComment,
	errs []error,
) error {
	pr := dst.(*bitBucketPR)
	if bb.maxComments > 0 && len(pendingComments) > bb.maxComments {
		if err := bb.postGeneralComment(ctx, pr, tooManyCommentsMsg(len(pendingComments), bb.maxComments)); err != nil {
			errs = append(errs, fmt.Errorf("failed to create general comment: %w", err))
		}
	}
	if len(errs) > 0 {
		if err := bb.postGeneralComment(ctx, pr, errsToComment(errs)); err != nil {
			return fmt.Errorf("failed to create general comment: %w", err)
		}
	}
	if details := makePrometheusDetailsComment(s); details != "" {
		if err := bb.postGeneralComment(ctx, pr, details); err != nil {
			return fmt.Errorf("failed to create general comment: %w", err)
		}
	}
	return nil
}

func (bb BitBucketReporter) List(ctx context.Context, dst any) ([]ExistingComment, error) {
	pr := dst.(*bitBucketPR)
	bbComments, err := bb.getPullRequestComments(ctx, pr)
	if err != nil {
		return nil, err
	}

	comments := make([]ExistingComment, 0, len(bbComments))
	for _, c := range bbComments {
		comments = append(comments, ExistingComment{
			id:   strconv.Itoa(c.id),
			path: c.anchor.Path,
			text: c.text,
			line: c.anchor.Line,
			meta: bitBucketCommentMeta{
				id:       c.id,
				version:  c.version,
				severity: c.severity,
				replies:  c.replies,
			},
			isGeneral: c.anchor.Path == "",
		})
	}
	return comments, nil
}

func (bb BitBucketReporter) Create(ctx context.Context, dst any, p PendingComment) error {
	pr := dst.(*bitBucketPR)
	comment := pendingToBitBucketComment(p)
	return bb.postComment(ctx, pr, comment)
}

func (bb BitBucketReporter) Delete(ctx context.Context, dst any, c ExistingComment) error {
	pr := dst.(*bitBucketPR)
	meta := c.meta.(bitBucketCommentMeta)
	bc := bitBucketComment{
		text:     "",
		severity: meta.severity,
		anchor: BitBucketCommentAnchor{
			Path:     "",
			Line:     0,
			Orphaned: false,
		},
		id:      meta.id,
		version: meta.version,
		replies: meta.replies,
	}
	switch {
	case bc.replies == 0:
		return bb.deleteComment(ctx, pr, bc)
	case bc.severity == "BLOCKER":
		return bb.resolveTask(ctx, pr, bc)
	default:
		if err := bb.updateSeverity(ctx, pr, bc, "BLOCKER"); err != nil {
			return err
		}
		return bb.resolveTask(ctx, pr, bc)
	}
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

func (bb BitBucketReporter) MaxComments() int {
	return bb.maxComments
}

// The generated comment must follow these rules:
// https://developer.atlassian.com/server/bitbucket/rest/v905/api-group-pull-requests/#api-api-latest-projects-projectkey-repos-repositoryslug-pull-requests-pullrequestid-comments-post
func pendingToBitBucketComment(p PendingComment) BitBucketPendingComment {
	comment := BitBucketPendingComment{
		Text:     p.text,
		Severity: "NORMAL",
		Anchor: BitBucketPendingCommentAnchor{
			Path:     p.path,
			Line:     p.line,
			DiffType: "EFFECTIVE",
			LineType: "CONTEXT",
			FileType: "FROM",
		},
	}

	switch {
	case p.isGeneral:
		comment.Anchor = BitBucketPendingCommentAnchor{
			Path:     "",
			LineType: "",
			FileType: "",
			DiffType: "EFFECTIVE",
			Line:     0,
		}
	case p.isBefore:
		comment.Anchor.LineType = "REMOVED"
	case p.isModified:
		comment.Anchor.LineType = "ADDED"
		comment.Anchor.FileType = "TO"
	default:
		if p.oldLine > 0 {
			comment.Anchor.Line = p.oldLine
		}
	}

	return comment
}

type bitBucketCommentMeta struct {
	severity string
	id       int
	version  int
	replies  int
}

type bitBucketPR struct {
	srcBranch string
	srcHead   string
	dstBranch string
	dstHead   string
	ID        int
}

type bitBucketComment struct {
	text     string
	severity string
	anchor   BitBucketCommentAnchor
	id       int
	version  int
	replies  int
}

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
	NextPageStart int                    `json:"nextPageStart"`
	IsLastPage    bool                   `json:"isLastPage"`
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
	NextPageStart int                            `json:"nextPageStart"`
	IsLastPage    bool                           `json:"isLastPage"`
}

type BitBucketPendingCommentAnchor struct {
	Path     string `json:"path,omitempty"`
	LineType string `json:"lineType,omitempty"`
	FileType string `json:"fileType,omitempty"`
	DiffType string `json:"diffType"`
	Line     int    `json:"line,omitempty"`
}

type BitBucketPendingComment struct {
	Text     string                        `json:"text"`
	Severity string                        `json:"severity"`
	Anchor   BitBucketPendingCommentAnchor `json:"anchor"`
}

type BitBucketCommentStateUpdate struct {
	State   string `json:"state"`
	Version int    `json:"version"`
}

type BitBucketCommentSeverityUpdate struct {
	Severity string `json:"severity"`
	Version  int    `json:"version"`
}

func (bb BitBucketReporter) request(ctx context.Context, method, path string, body io.Reader) ([]byte, error) {
	slog.LogAttrs(
		ctx, slog.LevelInfo,
		"Sending a request to BitBucket",
		slog.String("method", method),
		slog.String("path", path),
	)

	if body != nil {
		payload, _ := io.ReadAll(body)
		slog.LogAttrs(
			ctx, slog.LevelDebug,
			"Request payload",
			slog.String("body", string(payload)),
		)
		body = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, bb.uri+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+bb.authToken)

	netClient := &http.Client{Timeout: bb.timeout}

	resp, err := netClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return data, err
	}

	slog.LogAttrs(
		ctx, slog.LevelInfo,
		"BitBucket request completed",
		slog.Int("status", resp.StatusCode),
	)
	slog.LogAttrs(
		ctx, slog.LevelDebug,
		"BitBucket response body",
		slog.Int("code", resp.StatusCode),
		slog.String("body", string(data)),
	)
	if resp.StatusCode >= 300 {
		slog.LogAttrs(
			ctx, slog.LevelError,
			"Got a non 2xx response",
			slog.String("body", string(data)),
			slog.String("path", path),
			slog.Int("code", resp.StatusCode),
		)
		return data, fmt.Errorf("%s request failed", method)
	}

	return data, err
}

func (bb BitBucketReporter) whoami(ctx context.Context) (string, error) {
	resp, err := bb.request(ctx, http.MethodGet, "/plugins/servlet/applinks/whoami", nil)
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(string(resp), "\n"), nil
}

func (bb BitBucketReporter) findPullRequestForBranch(ctx context.Context, branch, commit string) (*bitBucketPR, error) {
	var start int
	for {
		resp, err := bb.request(
			ctx,
			http.MethodGet,
			fmt.Sprintf(
				"/rest/api/1.0/projects/%s/repos/%s/commits/%s/pull-requests?start=%d",
				bb.project, bb.repo, commit, start,
			),
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

func (bb BitBucketReporter) getPullRequestComments(ctx context.Context, pr *bitBucketPR) ([]bitBucketComment, error) {
	username, err := bb.whoami(ctx)
	if err != nil {
		return nil, err
	}

	comments := []bitBucketComment{}

	var start int
	for {
		resp, err := bb.request(
			ctx,
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
				id:       act.Comment.ID,
				version:  act.Comment.Version,
				text:     act.Comment.Text,
				anchor:   act.CommentAnchor,
				severity: act.Comment.Severity,
				replies:  len(act.Comment.Comments),
			})
		}

		if acts.IsLastPage || acts.NextPageStart == start {
			break
		}
		start = acts.NextPageStart
	}

	return comments, nil
}

func (bb BitBucketReporter) deleteComment(ctx context.Context, pr *bitBucketPR, cur bitBucketComment) error {
	slog.LogAttrs(
		ctx, slog.LevelDebug,
		"Deleting stale comment",
		slog.Int("id", cur.id),
	)
	_, err := bb.request(
		ctx,
		http.MethodDelete,
		fmt.Sprintf(
			"/rest/api/1.0/projects/%s/repos/%s/pull-requests/%d/comments/%d?version=%d",
			bb.project, bb.repo,
			pr.ID,
			cur.id, cur.version,
		),
		nil,
	)
	return err
}

func (bb BitBucketReporter) resolveTask(ctx context.Context, pr *bitBucketPR, cur bitBucketComment) error {
	slog.LogAttrs(
		ctx, slog.LevelDebug,
		"Resolving stale blocker comment",
		slog.Int("id", cur.id),
	)
	payload, _ := json.Marshal(BitBucketCommentStateUpdate{
		State:   "RESOLVED",
		Version: cur.version,
	})
	_, err := bb.request(
		ctx,
		http.MethodPut,
		fmt.Sprintf(
			"/rest/api/1.0/projects/%s/repos/%s/pull-requests/%d/comments/%d",
			bb.project, bb.repo,
			pr.ID,
			cur.id,
		),
		bytes.NewReader(payload),
	)
	return err
}

func (bb BitBucketReporter) updateSeverity(ctx context.Context, pr *bitBucketPR, cur bitBucketComment, severity string) error {
	slog.LogAttrs(
		ctx, slog.LevelDebug,
		"Updating comment severity",
		slog.Int("id", cur.id),
		slog.String("from", cur.severity),
		slog.String("to", severity),
	)
	payload, _ := json.Marshal(BitBucketCommentSeverityUpdate{
		Severity: severity,
		Version:  cur.version,
	})
	_, err := bb.request(
		ctx,
		http.MethodPut,
		fmt.Sprintf(
			"/rest/api/1.0/projects/%s/repos/%s/pull-requests/%d/comments/%d",
			bb.project, bb.repo,
			pr.ID,
			cur.id,
		),
		bytes.NewReader(payload),
	)
	return err
}

func (bb BitBucketReporter) postComment(ctx context.Context, pr *bitBucketPR, comment BitBucketPendingComment) error {
	payload, _ := json.Marshal(comment)
	_, err := bb.request(
		ctx,
		http.MethodPost,
		fmt.Sprintf(
			"/rest/api/1.0/projects/%s/repos/%s/pull-requests/%d/comments",
			bb.project, bb.repo,
			pr.ID,
		),
		bytes.NewReader(payload),
	)
	return err
}

func (bb BitBucketReporter) postGeneralComment(ctx context.Context, pr *bitBucketPR, text string) error {
	comment := BitBucketPendingComment{
		Text:     text,
		Severity: "NORMAL",
		Anchor: BitBucketPendingCommentAnchor{
			Path:     "",
			LineType: "",
			FileType: "",
			DiffType: "EFFECTIVE",
			Line:     0,
		},
	}
	return bb.postComment(ctx, pr, comment)
}
