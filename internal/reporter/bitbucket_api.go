package reporter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/cloudflare/pint/internal/checks"
)

type BitBucketReport struct {
	Reporter string                `json:"reporter"`
	Title    string                `json:"title"`
	Result   string                `json:"result"`
	Details  string                `json:"details"`
	Link     string                `json:"link"`
	Data     []BitBucketReportData `json:"data"`
}

type DataType string

const (
	BooleanType    DataType = "BOOLEAN"
	DateType       DataType = "DATA"
	DurationType   DataType = "DURATION"
	LinkType       DataType = "LINK"
	NumberType     DataType = "NUMBER"
	PercentageType DataType = "PERCENTAGE"
	TextType       DataType = "TEXT"

	maxCommentLength = 32768
)

type BitBucketReportData struct {
	Value any      `json:"value"`
	Title string   `json:"title"`
	Type  DataType `json:"type"`
}

type BitBucketAnnotation struct {
	Path     string `json:"path"`
	Message  string `json:"message"`
	Severity string `json:"severity"`
	Type     string `json:"type"`
	Link     string `json:"link"`
	Line     int    `json:"line"`
}

type BitBucketAnnotations struct {
	Annotations []BitBucketAnnotation `json:"annotations"`
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

type bitBucketPRChanges struct {
	pathModifiedLines map[string][]int
	pathLineMapping   map[string]map[int]int
}

type BitBucketPath struct {
	ToString string `json:"toString"`
}

type BitBucketPullRequestChange struct {
	Path BitBucketPath `json:"path"`
}

type BitBucketPullRequestChanges struct {
	Values        []BitBucketPullRequestChange `json:"values"`
	Start         int                          `json:"start"`
	NextPageStart int                          `json:"nextPageStart"`
	IsLastPage    bool                         `json:"isLastPage"`
}

type BitBucketDiffLine struct {
	Source      int `json:"source"`
	Destination int `json:"destination"`
}

type BitBucketDiffSegment struct {
	Type  string              `json:"type"`
	Lines []BitBucketDiffLine `json:"lines"`
}

type BitBucketDiffHunk struct {
	Segments []BitBucketDiffSegment `json:"segments"`
}

type BitBucketFileDiff struct {
	Hunks []BitBucketDiffHunk `json:"hunks"`
}

type BitBucketFileDiffs struct {
	Diffs []BitBucketFileDiff `json:"diffs"`
}

type bitBucketComment struct {
	text     string
	severity string
	anchor   BitBucketCommentAnchor
	id       int
	version  int
	replies  int
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

func (ba BitBucketCommentAnchor) isEqual(pa BitBucketPendingCommentAnchor) bool {
	if ba.Path != pa.Path {
		return false
	}
	if ba.Line != pa.Line {
		return false
	}
	if ba.LineType != pa.LineType {
		return false
	}
	if ba.DiffType != pa.DiffType {
		return false
	}
	return true
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

type pendingComment struct {
	severity string
	text     string
	path     string
	line     int
	anchor   checks.Anchor
}

func (pc pendingComment) toBitBucketComment(changes *bitBucketPRChanges) BitBucketPendingComment {
	c := BitBucketPendingComment{
		Anchor: BitBucketPendingCommentAnchor{
			Path:     pc.path,
			Line:     pc.line,
			DiffType: "EFFECTIVE",
			LineType: "CONTEXT",
			FileType: "FROM",
		},
		Text:     pc.text,
		Severity: pc.severity,
	}

	if pc.anchor == checks.AnchorBefore {
		c.Anchor.LineType = "REMOVED"
	} else if changes != nil {
		if lines, ok := changes.pathModifiedLines[pc.path]; ok && slices.Contains(lines, pc.line) {
			c.Anchor.LineType = "ADDED"
			c.Anchor.FileType = "TO"
		}
		if c.Anchor.FileType == "FROM" {
			if m, ok := changes.pathLineMapping[pc.path]; ok {
				if v, found := m[pc.line]; found {
					c.Anchor.Line = v
				}
			}
		}
	}

	return c
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

func newBitBucketAPI(pintVersion, uri string, timeout time.Duration, token, project, repo string, maxComments int) *bitBucketAPI {
	return &bitBucketAPI{
		pintVersion: pintVersion,
		uri:         uri,
		timeout:     timeout,
		authToken:   token,
		project:     project,
		repo:        repo,
		maxComments: maxComments,
	}
}

type bitBucketAPI struct {
	pintVersion string
	uri         string
	authToken   string
	project     string
	repo        string
	timeout     time.Duration
	maxComments int
}

func (bb bitBucketAPI) request(method, path string, body io.Reader) ([]byte, error) {
	slog.Info("Sending a request to BitBucket", slog.String("method", method), slog.String("path", path))

	if body != nil {
		payload, _ := io.ReadAll(body)
		slog.Debug("Request payload", slog.String("body", string(payload)))
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

	slog.Info("BitBucket request completed", slog.Int("status", resp.StatusCode))
	slog.Debug("BitBucket response body", slog.Int("code", resp.StatusCode), slog.String("body", string(data)))
	if resp.StatusCode >= 300 {
		slog.Error(
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

func (bb bitBucketAPI) deleteReport(commit string) error {
	_, err := bb.request(
		http.MethodDelete,
		fmt.Sprintf("/rest/insights/1.0/projects/%s/repos/%s/commits/%s/reports/pint", bb.project, bb.repo, commit),
		nil,
	)
	return err
}

func (bb bitBucketAPI) createReport(summary Summary, commit string) error {
	result := "PASS"
	var reportedProblems int
	for _, report := range summary.reports {
		reportedProblems++
		if report.Problem.Severity >= checks.Bug {
			result = "FAIL"
		}
	}

	payload, _ := json.Marshal(BitBucketReport{
		Title:    "pint " + bb.pintVersion,
		Result:   result,
		Reporter: "Prometheus rule linter",
		Details:  BitBucketDescription,
		Link:     "https://cloudflare.github.io/pint/",
		Data: []BitBucketReportData{
			{Title: "Number of rules parsed", Type: NumberType, Value: summary.TotalEntries},
			{Title: "Number of rules checked", Type: NumberType, Value: summary.CheckedEntries},
			{Title: "Number of problems found", Type: NumberType, Value: reportedProblems},
			{Title: "Number of offline checks", Type: NumberType, Value: summary.OfflineChecks},
			{Title: "Number of online checks", Type: NumberType, Value: summary.OnlineChecks},
			{Title: "Checks duration", Type: DurationType, Value: summary.Duration.Milliseconds()},
		},
	})

	_, err := bb.request(
		http.MethodPut,
		fmt.Sprintf("/rest/insights/1.0/projects/%s/repos/%s/commits/%s/reports/pint", bb.project, bb.repo, commit),
		bytes.NewReader(payload),
	)
	return err
}

func (bb bitBucketAPI) createAnnotations(summary Summary, commit string) error {
	annotations := make([]BitBucketAnnotation, 0, len(summary.reports))
	for _, report := range summary.reports {
		annotations = append(annotations, reportToAnnotation(report))
	}

	if len(annotations) == 0 {
		return nil
	}

	payload, _ := json.Marshal(BitBucketAnnotations{Annotations: annotations})
	_, err := bb.request(
		http.MethodPost,
		fmt.Sprintf("/rest/insights/1.0/projects/%s/repos/%s/commits/%s/reports/pint/annotations", bb.project, bb.repo, commit),
		bytes.NewReader(payload),
	)
	return err
}

func (bb bitBucketAPI) deleteAnnotations(commit string) error {
	_, err := bb.request(
		http.MethodDelete,
		fmt.Sprintf("/rest/insights/1.0/projects/%s/repos/%s/commits/%s/reports/pint/annotations", bb.project, bb.repo, commit),
		nil,
	)
	return err
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

func (bb bitBucketAPI) getPullRequestChanges(pr *bitBucketPR) (*bitBucketPRChanges, error) {
	prChanges := bitBucketPRChanges{
		pathModifiedLines: map[string][]int{},
		pathLineMapping:   map[string]map[int]int{},
	}

	var start int
	for {
		resp, err := bb.request(
			http.MethodGet,
			fmt.Sprintf("/rest/api/1.0/projects/%s/repos/%s/pull-requests/%d/changes?start=%d", bb.project, bb.repo, pr.ID, start),
			nil,
		)
		if err != nil {
			return nil, err
		}

		var changes BitBucketPullRequestChanges
		if err = json.Unmarshal(resp, &changes); err != nil {
			return nil, err
		}

		for _, ch := range changes.Values {
			modifiedLines, lineMap, err := bb.getFileDiff(pr, ch.Path.ToString)
			if err != nil {
				return nil, err
			}
			prChanges.pathModifiedLines[ch.Path.ToString] = modifiedLines
			prChanges.pathLineMapping[ch.Path.ToString] = lineMap
		}

		if changes.IsLastPage || changes.NextPageStart == start {
			break
		}
		start = changes.NextPageStart
	}

	return &prChanges, nil
}

func (bb bitBucketAPI) getFileDiff(pr *bitBucketPR, path string) ([]int, map[int]int, error) {
	resp, err := bb.request(
		http.MethodGet,
		fmt.Sprintf(
			"/rest/api/latest/projects/%s/repos/%s/commits/%s/diff/%s?contextLines=10000&since=%s&whitespace=show&withComments=false",
			bb.project, bb.repo,
			pr.srcHead,
			path,
			pr.dstHead,
		),
		nil,
	)
	if err != nil {
		return nil, nil, err
	}

	var fileDiffs BitBucketFileDiffs
	if err = json.Unmarshal(resp, &fileDiffs); err != nil {
		return nil, nil, err
	}

	modifiedLines := []int{}
	lineMap := map[int]int{}
	for _, diff := range fileDiffs.Diffs {
		for _, hunk := range diff.Hunks {
			for _, seg := range hunk.Segments {
				for _, line := range seg.Lines {
					if seg.Type == "ADDED" {
						modifiedLines = append(modifiedLines, line.Destination)
					}
					if seg.Type == "CONTEXT" || seg.Type == "ADDED" {
						lineMap[line.Destination] = line.Source
					}
				}
			}
		}
	}

	return modifiedLines, lineMap, nil
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

func (bb bitBucketAPI) makeComments(summary Summary, changes *bitBucketPRChanges) []BitBucketPendingComment {
	var buf strings.Builder
	comments := []BitBucketPendingComment{}
	for _, reports := range dedupReports(summary.reports) {
		if _, ok := changes.pathModifiedLines[reports[0].Path.SymlinkTarget]; !ok {
			continue
		}

		mergeDetails := identicalDetails(reports)

		buf.Reset()

		buf.WriteString(problemIcon(reports[0].Problem.Severity))
		buf.WriteString(" **")
		buf.WriteString(reports[0].Problem.Severity.String())
		buf.WriteString("** reported by [pint](https://cloudflare.github.io/pint/) **")
		buf.WriteString(reports[0].Problem.Reporter)
		buf.WriteString("** check.\n\n")
		for _, report := range reports {
			buf.WriteString("------\n\n")
			buf.WriteString(report.Problem.Text)
			buf.WriteString("\n\n")
			if !mergeDetails && report.Problem.Details != "" {
				buf.WriteString(report.Problem.Details)
				buf.WriteString("\n\n")
			}
			if report.Path.SymlinkTarget != report.Path.Name {
				buf.WriteString(":leftwards_arrow_with_hook: This problem was detected on a symlinked file ")
				buf.WriteRune('`')
				buf.WriteString(report.Path.Name)
				buf.WriteString("`.\n\n")
			}
		}
		if mergeDetails && reports[0].Problem.Details != "" {
			buf.WriteString("------\n\n")
			buf.WriteString(reports[0].Problem.Details)
			buf.WriteString("\n\n")
		}
		buf.WriteString("------\n\n")
		buf.WriteString(":information_source: To see documentation covering this check and instructions on how to resolve it [click here](https://cloudflare.github.io/pint/checks/")
		buf.WriteString(reports[0].Problem.Reporter)
		buf.WriteString(".html).\n")

		var severity string
		// nolint:exhaustive
		switch reports[0].Problem.Severity {
		case checks.Bug, checks.Fatal:
			severity = "BLOCKER"
		default:
			severity = "NORMAL"
		}

		var text string
		// BitBucket has a max comment length limit. If we hit it then truncate the comment.
		if buf.Len() > maxCommentLength {
			text = buf.String()[:maxCommentLength-4] + " ..."
		} else {
			text = buf.String()
		}

		pending := pendingComment{
			severity: severity,
			path:     reports[0].Path.SymlinkTarget,
			line:     reports[0].Problem.Lines.Last,
			text:     text,
			anchor:   reports[0].Problem.Anchor,
		}
		comments = append(comments, pending.toBitBucketComment(changes))
	}
	return comments
}

func (bb bitBucketAPI) limitComments(src []BitBucketPendingComment) []BitBucketPendingComment {
	if len(src) <= bb.maxComments {
		return src
	}
	comments := src[:bb.maxComments]
	comments = append(comments, BitBucketPendingComment{
		Text: fmt.Sprintf(`This pint run would create %d comment(s), which is more than %d limit configured for pint.
%d comments were skipped and won't be visibile on this PR.`, len(src), bb.maxComments, len(src)-bb.maxComments),
		Severity: "NORMAL",
		Anchor: BitBucketPendingCommentAnchor{ // nolint: exhaustruct
			DiffType: "EFFECTIVE",
		},
	})
	return comments
}

func (bb bitBucketAPI) pruneComments(pr *bitBucketPR, currentComments []bitBucketComment, pendingComments []BitBucketPendingComment) {
	for _, cur := range currentComments {
		slog.Debug(
			"Existing comment",
			slog.Int("id", cur.id),
			slog.Int("version", cur.version),
			slog.String("severity", cur.severity),
			slog.String("path", cur.anchor.Path),
			slog.Int("line", cur.anchor.Line),
			slog.String("diffType", cur.anchor.DiffType),
			slog.String("lineType", cur.anchor.LineType),
			slog.Bool("orphaned", cur.anchor.Orphaned),
			slog.Int("replies", cur.replies),
		)
		var keep bool
		for _, pend := range pendingComments {
			if cur.anchor.isEqual(pend.Anchor) && cur.text == pend.Text {
				keep = true
				break
			}
			if cur.anchor.DiffType == "COMMIT" {
				keep = false
			}
		}
		if !keep {
			switch {
			case cur.replies == 0:
				bb.deleteComment(pr, cur)
			case cur.severity == "BLOCKER":
				bb.resolveTask(pr, cur)
			default:
				bb.updateSeverity(pr, cur, "BLOCKER")
				bb.resolveTask(pr, cur)
			}
		}
	}
}

func (bb bitBucketAPI) deleteComment(pr *bitBucketPR, cur bitBucketComment) {
	slog.Debug(
		"Deleting stale comment",
		slog.Int("id", cur.id),
		slog.String("path", cur.anchor.Path),
		slog.Int("line", cur.anchor.Line),
	)
	_, err := bb.request(
		http.MethodDelete,
		fmt.Sprintf("/rest/api/1.0/projects/%s/repos/%s/pull-requests/%d/comments/%d?version=%d",
			bb.project, bb.repo,
			pr.ID,
			cur.id, cur.version,
		),
		nil,
	)
	if err != nil {
		slog.Error(
			"Failed to delete stale BitBucket pull request comment",
			slog.Int("id", cur.id),
			slog.Any("err", err),
		)
	}
}

func (bb bitBucketAPI) resolveTask(pr *bitBucketPR, cur bitBucketComment) {
	slog.Debug(
		"Resolving stale blocker comment",
		slog.Int("id", cur.id),
		slog.String("path", cur.anchor.Path),
		slog.Int("line", cur.anchor.Line),
	)
	payload, _ := json.Marshal(BitBucketCommentStateUpdate{
		State:   "RESOLVED",
		Version: cur.version,
	})
	_, err := bb.request(
		http.MethodPut,
		fmt.Sprintf("/rest/api/1.0/projects/%s/repos/%s/pull-requests/%d/comments/%d",
			bb.project, bb.repo,
			pr.ID,
			cur.id,
		),
		bytes.NewReader(payload),
	)
	if err != nil {
		slog.Error(
			"Failed to resolve stale blocker BitBucket pull request comment",
			slog.Int("id", cur.id),
			slog.Any("err", err),
		)
	}
}

func (bb bitBucketAPI) updateSeverity(pr *bitBucketPR, cur bitBucketComment, severity string) {
	slog.Debug(
		"Updating comment severity",
		slog.Int("id", cur.id),
		slog.String("path", cur.anchor.Path),
		slog.Int("line", cur.anchor.Line),
		slog.String("from", cur.severity),
		slog.String("to", severity),
	)
	payload, _ := json.Marshal(BitBucketCommentSeverityUpdate{
		Severity: severity,
		Version:  cur.version,
	})
	_, err := bb.request(
		http.MethodPut,
		fmt.Sprintf("/rest/api/1.0/projects/%s/repos/%s/pull-requests/%d/comments/%d",
			bb.project, bb.repo,
			pr.ID,
			cur.id,
		),
		bytes.NewReader(payload),
	)
	if err != nil {
		slog.Error(
			"Failed to update BitBucket pull request comment severity",
			slog.Int("id", cur.id),
			slog.Any("err", err),
		)
	}
}

func (bb bitBucketAPI) addComments(pr *bitBucketPR, currentComments []bitBucketComment, pendingComments []BitBucketPendingComment) error {
	var added int
	for _, pend := range pendingComments {
		add := true
		for _, cur := range currentComments {
			if cur.anchor.isEqual(pend.Anchor) && cur.text == pend.Text {
				add = false
			}
			if cur.anchor.DiffType == "COMMIT" {
				add = true
			}
		}
		if add {
			slog.Debug(
				"Adding missing comment",
				slog.String("path", pend.Anchor.Path),
				slog.Int("line", pend.Anchor.Line),
				slog.String("diffType", pend.Anchor.DiffType),
				slog.String("lineType", pend.Anchor.LineType),
				slog.String("fileType", pend.Anchor.FileType),
			)
			payload, _ := json.Marshal(pend)
			_, err := bb.request(
				http.MethodPost,
				fmt.Sprintf("/rest/api/1.0/projects/%s/repos/%s/pull-requests/%d/comments",
					bb.project, bb.repo,
					pr.ID,
				),
				bytes.NewReader(payload),
			)
			if err != nil {
				return err
			}
			added++
		}
	}
	slog.Info("Added pull request comments to BitBucket", slog.Int("count", added))
	return nil
}

func reportToAnnotation(report Report) BitBucketAnnotation {
	var msgPrefix, severity, atype string
	reportLine, srcLine := moveReportedLine(report)
	if reportLine != srcLine {
		msgPrefix = fmt.Sprintf("Problem reported on unmodified line %d, annotation moved here: ", srcLine)
	}
	if report.Path.SymlinkTarget != report.Path.Name {
		if msgPrefix == "" {
			msgPrefix = fmt.Sprintf("Problem detected on symlinked file %s: ", report.Path.Name)
		} else {
			msgPrefix = fmt.Sprintf("Problem detected on symlinked file %s. %s", report.Path.Name, msgPrefix)
		}
	}

	switch report.Problem.Severity {
	case checks.Fatal:
		severity = "HIGH"
		atype = "BUG"
	case checks.Bug:
		severity = "MEDIUM"
		atype = "BUG"
	case checks.Warning, checks.Information:
		severity = "LOW"
		atype = "CODE_SMELL"
	}

	return BitBucketAnnotation{
		Path:     report.Path.SymlinkTarget,
		Line:     reportLine,
		Message:  fmt.Sprintf("%s%s: %s", msgPrefix, report.Problem.Reporter, report.Problem.Text),
		Severity: severity,
		Type:     atype,
		Link:     fmt.Sprintf("https://cloudflare.github.io/pint/checks/%s.html", report.Problem.Reporter),
	}
}
