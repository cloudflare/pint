package reporter

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"time"

	gitlab "gitlab.com/gitlab-org/api/client-go"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/output"
)

type gitlabLogger struct{}

func (l gitlabLogger) Error(msg string, keysAndValues ...any) {
	slog.Error(msg, keysAndValuesToAttrs(keysAndValues...)...)
}

func (l gitlabLogger) Info(msg string, keysAndValues ...any) {
	slog.Info(msg, keysAndValuesToAttrs(keysAndValues...)...)
}

func (l gitlabLogger) Debug(msg string, keysAndValues ...any) {
	slog.Debug(msg, keysAndValuesToAttrs(keysAndValues...)...)
}

func (l gitlabLogger) Warn(msg string, keysAndValues ...any) {
	slog.Warn(msg, keysAndValuesToAttrs(keysAndValues...)...)
}

func keysAndValuesToAttrs(keysAndValues ...any) (attrs []any) {
	attrs = append(attrs, slog.String("reporter", "GitLab"))
	var key string
	var ok bool
	for i, elem := range keysAndValues {
		switch {
		case i%2 == 0:
			key, ok = elem.(string)
		case ok && key != "":
			attrs = append(attrs, slog.Any(key, elem))
		}
	}
	return attrs
}

type gitlabMR struct {
	version     *gitlab.MergeRequestDiffVersion
	diffs       []*gitlab.MergeRequestDiff
	discussions []*gitlab.Discussion
	userID      int
	mrID        int
}

func (glmr gitlabMR) String() string {
	return strconv.Itoa(glmr.mrID)
}

type gitlabComment struct {
	discussionID string
	baseSHA      string
	headSHA      string
	startSHA     string
	noteID       int
}

type GitLabReporter struct {
	client      *gitlab.Client
	version     string
	branch      string
	timeout     time.Duration
	project     int
	maxComments int
}

func NewGitLabReporter(version, branch, uri string, timeout time.Duration, token string, project, maxComments int) (_ GitLabReporter, err error) {
	slog.Info(
		"Will report problems to GitLab",
		slog.String("uri", uri),
		slog.String("timeout", output.HumanizeDuration(timeout)),
		slog.String("branch", branch),
		slog.Int("project", project),
		slog.Int("maxComments", maxComments),
	)
	gl := GitLabReporter{
		client:      nil,
		timeout:     timeout,
		version:     version,
		branch:      branch,
		project:     project,
		maxComments: maxComments,
	}

	options := []gitlab.ClientOptionFunc{
		gitlab.WithCustomLeveledLogger(gitlabLogger{}),
	}
	if uri != "" {
		options = append(options, gitlab.WithBaseURL(uri+"/api/v4"))
	}

	gl.client, err = gitlab.NewClient(token, options...)
	if err != nil {
		return gl, fmt.Errorf("failed to create a new GitLab client: %w", err)
	}
	return gl, nil
}

func (gl GitLabReporter) Describe() string {
	return "GitLab"
}

func (gl GitLabReporter) Destinations(ctx context.Context) ([]any, error) {
	userID, err := gl.getUserID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get GitLab user ID: %w", err)
	}

	ids, err := gl.getMRs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get GitLab merge request details: %w", err)
	}

	dsts := make([]any, 0, len(ids))
	for _, id := range ids {
		slog.Info("Found open GitLab merge request", slog.String("branch", gl.branch), slog.Int("id", id))
		dst := gitlabMR{
			version:     nil,
			diffs:       nil,
			discussions: nil,
			userID:      userID,
			mrID:        id,
		}
		if dst.version, err = gl.getVersions(ctx, id); err != nil {
			return nil, fmt.Errorf("failed to get GitLab merge request versions: %w", err)
		}
		if dst.diffs, err = gl.getDiffs(ctx, id); err != nil {
			return nil, fmt.Errorf("failed to get GitLab merge request changes: %w", err)
		}
		if dst.discussions, err = gl.getDiscussions(ctx, id); err != nil {
			return nil, fmt.Errorf("failed to get GitLab merge request discussions: %w", err)
		}
		dsts = append(dsts, dst)
	}
	return dsts, nil
}

func (gl GitLabReporter) Summary(ctx context.Context, dst any, s Summary, pendingComments []PendingComment, errs []error) (err error) {
	mr := dst.(gitlabMR)
	if gl.maxComments > 0 && len(pendingComments) > gl.maxComments {
		if err = gl.generalComment(ctx, mr, tooManyCommentsMsg(len(pendingComments), gl.maxComments)); err != nil {
			errs = append(errs, fmt.Errorf("failed to create general comment: %w", err))
		}
	}
	if len(errs) > 0 {
		if err = gl.generalComment(ctx, mr, errsToComment(errs)); err != nil {
			return fmt.Errorf("failed to create general comment: %w", err)
		}
	}
	if details := makePrometheusDetailsComment(s); details != "" {
		if err = gl.generalComment(ctx, mr, details); err != nil {
			return fmt.Errorf("failed to create general comment: %w", err)
		}
	}
	return nil
}

func (gl GitLabReporter) List(_ context.Context, dst any) ([]ExistingComment, error) {
	mr := dst.(gitlabMR)
	comments := make([]ExistingComment, 0, len(mr.discussions))
	for _, disc := range mr.discussions {
		var c ExistingComment
		for _, note := range disc.Notes {
			if note.System {
				goto NEXT
			}
			if note.Author.ID != mr.userID {
				goto NEXT
			}
			if note.Position == nil {
				goto NEXT
			}
			if note.Resolved {
				goto NEXT
			}
			c = gl.noteToExisting(disc.ID, note)
			break
		}
		if c.path != "" && c.line > 0 && c.meta != nil {
			comments = append(comments, c)
		}
	NEXT:
	}
	return comments, nil
}

func (gl GitLabReporter) Create(ctx context.Context, dst any, comment PendingComment) error {
	unresolved, err := gl.unresolveIfPresent(ctx, dst, comment)
	if err != nil {
		slog.Warn("Failed to un-resolved existing comment, will create a new one",
			slog.String("path", comment.path),
			slog.Int("line", comment.line),
			slog.Any("err", err),
		)
	}
	if unresolved {
		slog.Debug("Existing comment was un-resolved, skipping creating new one")
		return nil
	}

	mr := dst.(gitlabMR)
	opt := reportToGitLabDiscussion(comment, mr.diffs, mr.version)
	if opt == nil {
		return nil
	}
	slog.Debug("Creating a new merge request discussion", loggifyDiscussion(opt)...)
	reqCtx, cancel := context.WithTimeout(ctx, gl.timeout)
	defer cancel()
	_, _, err = gl.client.Discussions.CreateMergeRequestDiscussion(gl.project, mr.mrID, opt, gitlab.WithContext(reqCtx))
	return err
}

func (gl GitLabReporter) Delete(ctx context.Context, dst any, comment ExistingComment) error {
	mr := dst.(gitlabMR)
	c := comment.meta.(gitlabComment)
	slog.Debug("Deleting stale merge request discussion note",
		slog.String("discussion", c.discussionID),
		slog.Int("note", c.noteID),
	)
	reqCtx, cancel := context.WithTimeout(ctx, gl.timeout)
	defer cancel()

	_, _, err := gl.client.Discussions.ResolveMergeRequestDiscussion(
		gl.project,
		mr.mrID,
		c.discussionID,
		&gitlab.ResolveMergeRequestDiscussionOptions{Resolved: gitlab.Ptr(true)},
		gitlab.WithContext(reqCtx),
	)
	return err
}

func (gl GitLabReporter) IsEqual(_ any, existing ExistingComment, pending PendingComment) bool {
	if existing.path != pending.path {
		return false
	}
	if existing.line != pending.line {
		return false
	}
	return strings.Trim(existing.text, "\n") == strings.Trim(pending.text, "\n")
}

func (gl GitLabReporter) CanDelete(ExistingComment) bool {
	return true
}

func (gl GitLabReporter) CanCreate(done int) bool {
	return done < gl.maxComments
}

func (gl *GitLabReporter) getUserID(ctx context.Context) (int, error) {
	slog.Debug("Getting current GitLab user details")
	ctx, cancel := context.WithTimeout(ctx, gl.timeout)
	defer cancel()
	user, _, err := gl.client.Users.CurrentUser(gitlab.WithContext(ctx))
	if err != nil {
		return 0, err
	}
	return user.ID, nil
}

func (gl *GitLabReporter) getMRs(ctx context.Context) (ids []int, err error) {
	slog.Debug("Finding merge requests for current branch", slog.String("branch", gl.branch))
	mrs, _, err := getGitLabPaginated(func(pageNum int) ([]*gitlab.BasicMergeRequest, *gitlab.Response, error) {
		reqCtx, cancel := context.WithTimeout(ctx, gl.timeout)
		defer cancel()
		return gl.client.MergeRequests.ListProjectMergeRequests(gl.project, &gitlab.ListProjectMergeRequestsOptions{
			State:        gitlab.Ptr("opened"),
			SourceBranch: gitlab.Ptr(gl.branch),
			ListOptions:  gitlab.ListOptions{Page: pageNum},
		}, gitlab.WithContext(reqCtx))
	})
	if err != nil {
		return nil, err
	}
	for _, mr := range mrs {
		ids = append(ids, mr.IID)
	}
	return ids, nil
}

func (gl *GitLabReporter) getDiffs(ctx context.Context, mrNum int) ([]*gitlab.MergeRequestDiff, error) {
	slog.Debug("Getting the list of merge request diffs", slog.Int("mr", mrNum))
	diffs, _, err := getGitLabPaginated(func(pageNum int) ([]*gitlab.MergeRequestDiff, *gitlab.Response, error) {
		reqCtx, cancel := context.WithTimeout(ctx, gl.timeout)
		defer cancel()
		return gl.client.MergeRequests.ListMergeRequestDiffs(gl.project, mrNum, &gitlab.ListMergeRequestDiffsOptions{
			ListOptions: gitlab.ListOptions{Page: pageNum},
		}, gitlab.WithContext(reqCtx))
	})
	return diffs, err
}

func (gl *GitLabReporter) getVersions(ctx context.Context, mrNum int) (*gitlab.MergeRequestDiffVersion, error) {
	slog.Debug("Getting the list of merge request versions", slog.Int("mr", mrNum))
	vers, _, err := getGitLabPaginated(func(pageNum int) ([]*gitlab.MergeRequestDiffVersion, *gitlab.Response, error) {
		reqCtx, cancel := context.WithTimeout(ctx, gl.timeout)
		defer cancel()
		return gl.client.MergeRequests.GetMergeRequestDiffVersions(gl.project, mrNum, &gitlab.GetMergeRequestDiffVersionsOptions{
			Page: pageNum,
		}, gitlab.WithContext(reqCtx))
	})
	if err != nil {
		return nil, err
	}
	if len(vers) == 0 {
		return nil, errors.New("no merge request versions found")
	}
	return vers[0], nil
}

func (gl *GitLabReporter) getDiscussions(ctx context.Context, mrNum int) ([]*gitlab.Discussion, error) {
	slog.Debug("Getting the list of merge request discussions", slog.Int("mr", mrNum))
	discs, _, err := getGitLabPaginated(func(pageNum int) ([]*gitlab.Discussion, *gitlab.Response, error) {
		reqCtx, cancel := context.WithTimeout(ctx, gl.timeout)
		defer cancel()
		return gl.client.Discussions.ListMergeRequestDiscussions(gl.project, mrNum, &gitlab.ListMergeRequestDiscussionsOptions{
			Page: pageNum,
		}, gitlab.WithContext(reqCtx))
	})
	return discs, err
}

func (gl GitLabReporter) generalComment(ctx context.Context, mr gitlabMR, msg string) (err error) {
	slog.Debug("Creating a PR comment", slog.String("body", msg))

	for _, disc := range mr.discussions {
		for _, note := range disc.Notes {
			if note.System {
				continue
			}
			if note.Author.ID != mr.userID {
				continue
			}
			if note.Position != nil {
				continue
			}
			if note.Body == msg {
				slog.Debug("Comment already exits", slog.String("body", msg))
				return nil
			}
		}
	}

	reqCtx, cancel := context.WithTimeout(ctx, gl.timeout)
	defer cancel()
	opt := gitlab.CreateMergeRequestDiscussionOptions{Body: gitlab.Ptr(msg)}
	_, _, err = gl.client.Discussions.CreateMergeRequestDiscussion(gl.project, mr.mrID, &opt, gitlab.WithContext(reqCtx))
	return err
}

func (gl GitLabReporter) noteToExisting(discID string, note *gitlab.Note) (c ExistingComment) {
	if note.Position.NewPath != "" {
		c.path = note.Position.NewPath
	} else {
		c.path = note.Position.OldPath
	}
	if note.Position.NewLine > 0 {
		c.line = note.Position.NewLine
	} else {
		c.line = note.Position.OldLine
	}
	c.text = note.Body
	c.meta = gitlabComment{
		discussionID: discID,
		noteID:       note.ID,
		baseSHA:      note.Position.BaseSHA,
		headSHA:      note.Position.HeadSHA,
		startSHA:     note.Position.StartSHA,
	}
	return c
}

func (gl GitLabReporter) unresolveIfPresent(ctx context.Context, dst any, comment PendingComment) (_ bool, err error) {
	mr := dst.(gitlabMR)
	for _, disc := range mr.discussions {
		var c ExistingComment
		for _, note := range disc.Notes {
			if note.System {
				continue
			}
			if note.Author.ID != mr.userID {
				continue
			}
			if note.Position == nil {
				continue
			}
			if !note.Resolved {
				continue
			}
			c = gl.noteToExisting(disc.ID, note)
			if gl.IsEqual(dst, c, comment) {
				meta := c.meta.(gitlabComment)
				slog.Debug("Un-resolving merge request discussion note",
					slog.String("discussion", meta.discussionID),
					slog.Int("note", meta.noteID),
				)
				reqCtx, cancel := context.WithTimeout(ctx, gl.timeout)
				defer cancel()

				_, _, err = gl.client.Discussions.ResolveMergeRequestDiscussion(
					gl.project,
					mr.mrID,
					meta.discussionID,
					&gitlab.ResolveMergeRequestDiscussionOptions{Resolved: gitlab.Ptr(false)},
					gitlab.WithContext(reqCtx),
				)
				return err == nil, err
			}
		}
	}
	return false, nil
}

func reportToGitLabDiscussion(pending PendingComment, diffs []*gitlab.MergeRequestDiff, ver *gitlab.MergeRequestDiffVersion) *gitlab.CreateMergeRequestDiscussionOptions {
	pathDiffs := getDiffsForPath(diffs, pending.path)
	if len(pathDiffs) == 0 {
		slog.Debug("Skipping report for path with no GitLab diff",
			slog.String("path", pending.path),
		)
		return nil
	}

	d := gitlab.CreateMergeRequestDiscussionOptions{
		Body: gitlab.Ptr(pending.text),
		Position: &gitlab.PositionOptions{
			PositionType: gitlab.Ptr("text"),
			BaseSHA:      gitlab.Ptr(ver.BaseCommitSHA),
			HeadSHA:      gitlab.Ptr(ver.HeadCommitSHA),
			StartSHA:     gitlab.Ptr(ver.StartCommitSHA),
		},
	}

	var renamed bool
	for _, diff := range pathDiffs {
		d.Position.OldPath = gitlab.Ptr(diff.OldPath)
		if d.Position.NewPath == nil {
			d.Position.NewPath = gitlab.Ptr(diff.NewPath)
		}
		if diff.OldPath != diff.NewPath {
			renamed = true
		}

		dl, ok := diffLineFor(parseDiffLines(diff.Diff), pending.line)
		switch {
		case !ok:
			// No diffLine for this line, could be a file rename.
			d.Position.NewLine = gitlab.Ptr(pending.line)
			d.Position.OldLine = gitlab.Ptr(pending.line)
		case pending.anchor == checks.AnchorBefore:
			// Comment on removed line.
			d.Position.OldLine = gitlab.Ptr(dl.old)
		case ok && !dl.wasModified:
			// Comment on unmodified line.
			d.Position.NewLine = gitlab.Ptr(dl.new)
			d.Position.OldLine = gitlab.Ptr(dl.old)
		default:
			// Comment on new or modified line.
			d.Position.NewLine = gitlab.Ptr(dl.new)
		}
	}
	if renamed {
		d.Position.OldLine = nil
	}

	return &d
}

func getDiffsForPath(diffs []*gitlab.MergeRequestDiff, path string) (changes []*gitlab.MergeRequestDiff) {
	for _, change := range diffs {
		if change.NewPath == path {
			changes = append(changes, change)
		}
	}
	return changes
}

type diffLine struct {
	old         int
	new         int
	wasModified bool
}

func diffLineFor(lines []diffLine, line int) (diffLine, bool) {
	if len(lines) == 0 {
		return diffLine{old: 0, new: 0, wasModified: false}, false
	}

	for i, dl := range lines {
		if dl.new == line {
			return dl, true
		}
		// Calculate unmodified line that does not present in the diff
		if dl.new > line {
			lastLines := dl
			if i > 0 {
				lastLines = lines[i-1]
			}
			gap := line - lastLines.new
			return diffLine{
				old:         lastLines.old + gap,
				new:         line,
				wasModified: false,
			}, true
		}
	}
	// Calculate unmodified line that is greater than the last diff line
	lastLines := lines[len(lines)-1]
	if line > lastLines.new {
		gap := line - lastLines.new
		return diffLine{
			old:         lastLines.old + gap,
			new:         line,
			wasModified: false,
		}, true
	}
	return diffLine{old: 0, new: 0, wasModified: false}, false
}

var diffRe = regexp.MustCompile(`@@ \-(\d+),(\d+) \+(\d+),(\d+) @@`)

func parseDiffLines(diff string) (lines []diffLine) {
	var oldLine, newLine int

	sc := bufio.NewScanner(strings.NewReader(diff))
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "@@"):
			matches := diffRe.FindStringSubmatch(line)
			if len(matches) == 5 {
				oldLine, _ = strconv.Atoi(matches[1])
				oldLine--
				newLine, _ = strconv.Atoi(matches[3])
				newLine--
			}
		case strings.HasPrefix(line, "--- "):
		case strings.HasPrefix(line, "+++ "):
		case strings.HasPrefix(line, "-"):
			oldLine++
		case strings.HasPrefix(line, "+"):
			newLine++
			lines = append(lines, diffLine{old: oldLine, new: newLine, wasModified: true})
		default:
			oldLine++
			newLine++
			lines = append(lines, diffLine{old: oldLine, new: newLine, wasModified: false})
		}
	}

	return lines
}

func getGitLabPaginated[T any](searchFunc func(pageNum int) ([]T, *gitlab.Response, error)) ([]T, *gitlab.Response, error) {
	items := []T{}
	pageNum := 1
	for {
		tempItems, response, err := searchFunc(pageNum)
		if err != nil {
			return nil, response, err
		}
		items = append(items, tempItems...)
		if response.NextPage == 0 {
			break
		}
		pageNum = response.NextPage
	}
	return items, nil, nil
}

func loggifyDiscussion(opt *gitlab.CreateMergeRequestDiscussionOptions) (attrs []any) {
	if opt.Position == nil {
		return nil
	}
	if opt.Position.BaseSHA != nil {
		attrs = append(attrs, slog.String("base_sha", *opt.Position.BaseSHA))
	}
	if opt.Position.HeadSHA != nil {
		attrs = append(attrs, slog.String("head_sha", *opt.Position.HeadSHA))
	}
	if opt.Position.StartSHA != nil {
		attrs = append(attrs, slog.String("start_sha", *opt.Position.StartSHA))
	}
	if opt.Position.OldPath != nil {
		attrs = append(attrs, slog.String("old_path", *opt.Position.OldPath))
	}
	if opt.Position.NewPath != nil {
		attrs = append(attrs, slog.String("new_path", *opt.Position.NewPath))
	}
	if opt.Position.OldLine != nil {
		attrs = append(attrs, slog.Int("old_line", *opt.Position.OldLine))
	}
	if opt.Position.NewLine != nil {
		attrs = append(attrs, slog.Int("new_line", *opt.Position.NewLine))
	}
	return attrs
}

func tooManyCommentsMsg(nr, m int) string {
	return fmt.Sprintf(`This pint run would create %d comment(s), which is more than the limit configured for pint (%d).
%d comment(s) were skipped and won't be visibile on this PR.`, nr, m, nr-m)
}
