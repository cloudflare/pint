package reporter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/git"
	"github.com/cloudflare/pint/internal/output"

	"github.com/rs/zerolog/log"
)

const (
	BitBucketDescription = "pint is a Prometheus rule linter/validator.\n" +
		"It will inspect all Prometheus recording and alerting rules for problems that could prevent these from working correctly.\n" +
		"Checks can be either offline (static checks using only rule definition) or online (validate rule against live Prometheus server)."
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
)

type BitBucketReportData struct {
	Title string   `json:"title"`
	Type  DataType `json:"type"`
	Value any      `json:"value"`
}

type BitBucketAnnotation struct {
	Path     string `json:"path"`
	Line     int    `json:"line"`
	Message  string `json:"message"`
	Severity string `json:"severity"`
	Type     string `json:"type"`
	Link     string `json:"link"`
}

type BitBucketAnnotations struct {
	Annotations []BitBucketAnnotation `json:"annotations"`
}

func NewBitBucketReporter(version, uri string, timeout time.Duration, token, project, repo string, gitCmd git.CommandRunner) BitBucketReporter {
	return BitBucketReporter{
		version:   version,
		uri:       uri,
		timeout:   timeout,
		authToken: token,
		project:   project,
		repo:      repo,
		gitCmd:    gitCmd,
	}
}

// BitBucketReporter send linter results to BitBucket using
// https://docs.atlassian.com/bitbucket-server/rest/7.8.0/bitbucket-code-insights-rest.html
type BitBucketReporter struct {
	version   string
	uri       string
	timeout   time.Duration
	authToken string
	project   string
	repo      string
	gitCmd    git.CommandRunner
}

func (r BitBucketReporter) Submit(summary Summary) (err error) {
	headCommit, err := git.HeadCommit(r.gitCmd)
	if err != nil {
		return fmt.Errorf("failed to get HEAD commit: %w", err)
	}
	log.Info().Str("commit", headCommit).Msg("Got HEAD commit from git")

	pb, err := blameReports(summary.Reports(), r.gitCmd)
	if err != nil {
		return fmt.Errorf("failed to run git blame: %w", err)
	}

	annotations := []BitBucketAnnotation{}
	for _, report := range summary.Reports() {
		annotations = append(annotations, r.makeAnnotation(report, pb)...)
	}

	isPassing := true
	for _, ann := range annotations {
		if ann.Type == "BUG" {
			isPassing = false
			break
		}
	}

	if err = r.postReport(headCommit, isPassing, annotations, summary); err != nil {
		return err
	}

	if summary.HasFatalProblems() {
		return fmt.Errorf("fatal error(s) reported")
	}

	return nil
}

func (r BitBucketReporter) makeAnnotation(report Report, pb git.FileBlames) (annotations []BitBucketAnnotation) {
	if !shouldReport(report) {
		log.Debug().
			Str("path", report.Path).
			Str("lines", output.FormatLineRangeString(report.Problem.Lines)).
			Msg("Problem reported on unmodified line, skipping")
		return
	}

	reportLine := reportedLine(report)

	var severity, atype string
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

	a := BitBucketAnnotation{
		Path:     report.Path,
		Line:     reportLine,
		Message:  fmt.Sprintf("%s: %s", report.Problem.Reporter, report.Problem.Text),
		Severity: severity,
		Type:     atype,
		Link:     fmt.Sprintf("https://cloudflare.github.io/pint/checks/%s.html", report.Problem.Reporter),
	}
	annotations = append(annotations, a)

	return
}

func (r BitBucketReporter) bitBucketRequest(method, url string, body []byte) error {
	log.Debug().Str("url", url).Str("method", method).Msg("Sending a request to BitBucket")
	log.Debug().Bytes("body", body).Msg("Request payload")
	req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", r.authToken))

	netClient := &http.Client{
		Timeout: r.timeout,
	}

	resp, err := netClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	log.Debug().Int("status", resp.StatusCode).Msg("BitBucket request completed")
	if resp.StatusCode >= 300 {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Error().Err(err).Msg("Failed to read response body")
		}
		log.Error().Bytes("body", body).Str("url", url).Int("code", resp.StatusCode).Msg("Got a non 2xx response")
		return fmt.Errorf("%s request failed", method)
	}

	return nil
}

func (r BitBucketReporter) createAnnotations(commit string, annotations []BitBucketAnnotation) error {
	payload, _ := json.Marshal(BitBucketAnnotations{Annotations: annotations})
	url := fmt.Sprintf("%s/rest/insights/1.0/projects/%s/repos/%s/commits/%s/reports/pint/annotations",
		r.uri, r.project, r.repo, commit)
	return r.bitBucketRequest(http.MethodPost, url, payload)
}

func (r BitBucketReporter) deleteAnnotations(commit string) error {
	url := fmt.Sprintf("%s/rest/insights/1.0/projects/%s/repos/%s/commits/%s/reports/pint/annotations",
		r.uri, r.project, r.repo, commit)
	return r.bitBucketRequest(http.MethodDelete, url, nil)
}

func (r BitBucketReporter) postReport(commit string, isPassing bool, annotations []BitBucketAnnotation, summary Summary) error {
	result := "PASS"
	if !isPassing {
		result = "FAIL"
	}
	payload, _ := json.Marshal(BitBucketReport{
		Title:    fmt.Sprintf("pint %s", r.version),
		Result:   result,
		Reporter: "Prometheus rule linter",
		Details:  BitBucketDescription,
		Link:     "https://cloudflare.github.io/pint/",
		Data: []BitBucketReportData{
			{Title: "Number of rules checked", Type: NumberType, Value: summary.Entries},
			{Title: "Number of problems found", Type: NumberType, Value: len(annotations)},
			{Title: "Number of offline checks", Type: NumberType, Value: summary.OfflineChecks},
			{Title: "Number of online checks", Type: NumberType, Value: summary.OnlineChecks},
			{Title: "Checks duration", Type: DurationType, Value: summary.Duration.Milliseconds()},
		},
	})

	url := fmt.Sprintf("%s/rest/insights/1.0/projects/%s/repos/%s/commits/%s/reports/pint",
		r.uri, r.project, r.repo, commit)
	if err := r.bitBucketRequest(http.MethodPut, url, payload); err != nil {
		return fmt.Errorf("failed to create BitBucket report: %w", err)
	}

	// Try to delete annotations when that happens so we don't end up with stale data if we run
	// pint twice, first with problems found, and second without any.
	if err := r.deleteAnnotations(commit); err != nil {
		return err
	}

	// BitBucket API requires at least one annotation, if there aren't any report is PASS anyway
	if len(annotations) == 0 {
		return nil
	}

	return r.createAnnotations(commit, annotations)
}
