package checks

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"time"

	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"
)

const (
	RuleLinkCheckName = "rule/link"
)

func NewRuleLinkCheck(re *TemplatedRegexp, uriRewrite string, timeout time.Duration, headers map[string]string, comment string, s Severity) RuleLinkCheck {
	return RuleLinkCheck{
		scheme:     []string{"http", "https"},
		re:         re,
		uriRewrite: uriRewrite,
		timeout:    timeout,
		headers:    headers,
		comment:    comment,
		severity:   s,
		instance:   fmt.Sprintf("%s(%s)", RuleLinkCheckName, re.anchored),
	}
}

type RuleLinkCheck struct {
	re         *TemplatedRegexp
	headers    map[string]string
	uriRewrite string
	comment    string
	instance   string
	scheme     []string
	timeout    time.Duration
	severity   Severity
}

func (c RuleLinkCheck) Meta() CheckMeta {
	return CheckMeta{
		States: []discovery.ChangeType{
			discovery.Noop,
			discovery.Added,
			discovery.Modified,
			discovery.Moved,
		},
		Online:        true,
		AlwaysEnabled: false,
	}
}

func (c RuleLinkCheck) String() string {
	return c.instance
}

func (c RuleLinkCheck) Reporter() string {
	return RuleLinkCheckName
}

func (c RuleLinkCheck) Check(ctx context.Context, entry *discovery.Entry, _ []*discovery.Entry) (problems []Problem) {
	if entry.Rule.AlertingRule == nil || entry.Rule.AlertingRule.Annotations == nil {
		return nil
	}

	var u *url.URL
	var err error
	var uri string
	var re *regexp.Regexp
	for _, ann := range entry.Rule.AlertingRule.Annotations.Items {
		u, err = url.Parse(ann.Value.Value)
		if err != nil {
			continue
		}

		if !slices.Contains(c.scheme, u.Scheme) {
			continue
		}

		re = c.re.MustExpand(entry.Rule)
		if !re.MatchString(u.String()) {
			continue
		}

		uri = u.String()
		slog.LogAttrs(ctx, slog.LevelDebug, "Found link to check", slog.String("link", uri))
		if c.uriRewrite != "" {
			var result []byte
			for _, submatches := range re.FindAllStringSubmatchIndex(uri, -1) {
				result = re.ExpandString(result, c.uriRewrite, uri, submatches)
			}
			uri = string(result)
			slog.LogAttrs(ctx, slog.LevelDebug, "Link URI rewritten by rule", slog.String("link", u.String()), slog.String("uri", uri))
		}

		if problem := c.checkLink(ctx, ann, uri); problem != nil {
			problems = append(problems, *problem)
		}
	}

	return problems
}

func (c RuleLinkCheck) checkLink(ctx context.Context, ann *parser.YamlKeyValue, uri string) *Problem {
	rctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	req, _ := http.NewRequestWithContext(rctx, http.MethodGet, uri, nil)

	for k, v := range c.headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.LogAttrs(ctx, slog.LevelDebug, "Link request returned an error", slog.String("uri", uri), slog.Any("err", err))
		return &Problem{
			Anchor: AnchorAfter,
			Lines: diags.LineRange{
				First: ann.Key.Pos.Lines().First,
				Last:  ann.Value.Pos.Lines().Last,
			},
			Reporter: c.Reporter(),
			Summary:  "link check failed",
			Details:  maybeComment(c.comment),
			Diagnostics: []diags.Diagnostic{
				{
					Message:     fmt.Sprintf("GET request for %s returned an error: %s.", uri, err),
					Pos:         ann.Value.Pos,
					FirstColumn: 1,
					LastColumn:  len(ann.Value.Value),
					Kind:        diags.Issue,
				},
			},
			Severity: c.severity,
		}
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.LogAttrs(ctx, slog.LevelDebug, "Link request returned invalid status code", slog.String("uri", uri), slog.String("status", resp.Status))
		return &Problem{
			Anchor: AnchorAfter,
			Lines: diags.LineRange{
				First: ann.Key.Pos.Lines().First,
				Last:  ann.Value.Pos.Lines().Last,
			},
			Reporter: c.Reporter(),
			Summary:  "link check failed",
			Details:  maybeComment(c.comment),
			Diagnostics: []diags.Diagnostic{
				{
					Message:     fmt.Sprintf("GET request for %s returned invalid status code: `%s`.", uri, resp.Status),
					Pos:         ann.Value.Pos,
					FirstColumn: 1,
					LastColumn:  len(ann.Value.Value),
					Kind:        diags.Issue,
				},
			},
			Severity: c.severity,
		}
	}
	slog.LogAttrs(ctx, slog.LevelDebug, "Link request returned a valid status code", slog.String("uri", uri), slog.String("status", resp.Status))
	return nil
}
