package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/prometheus/model/rulefmt"
	"go.uber.org/atomic"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/comments"
	"github.com/cloudflare/pint/internal/config"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/promapi"
	"github.com/cloudflare/pint/internal/reporter"
)

var (
	yamlErrRe          = regexp.MustCompile("^yaml: line (.+): (.+)")
	yamlUnmarshalErrRe = regexp.MustCompile("^yaml: unmarshal errors:\n  line (.+): (.+)")
	rulefmtGroupRe     = regexp.MustCompile("^([0-9]+):[0-9]+: group \".+\", rule [0-9]+, (.+)")
	rulefmtGroupnameRe = regexp.MustCompile("^([0-9]+):[0-9]+: (groupname: .+)")
)

const (
	yamlParseReporter   = "yaml/parse"
	ignoreFileReporter  = "ignore/file"
	pintCommentReporter = "pint/comment"
)

func tryDecodingYamlError(err error) (l int, s string) {
	s = err.Error()

	werr := &rulefmt.WrappedError{}
	if errors.As(err, &werr) {
		if uerr := werr.Unwrap(); uerr != nil {
			s = uerr.Error()
		}
	}

	for _, re := range []*regexp.Regexp{yamlErrRe, yamlUnmarshalErrRe, rulefmtGroupRe, rulefmtGroupnameRe} {
		parts := re.FindStringSubmatch(err.Error())
		if len(parts) > 2 {
			line, err2 := strconv.Atoi(parts[1])
			if err2 != nil || line <= 0 {
				return 1, s
			}
			return line, parts[2]
		}
	}
	return 1, s
}

func checkRules(ctx context.Context, workers int, isOffline bool, gen *config.PrometheusGenerator, cfg config.Config, entries []discovery.Entry) (summary reporter.Summary, err error) {
	if isOffline {
		slog.Info("Offline mode, skipping Prometheus discovery")
	} else {
		if len(entries) > 0 {
			if err = gen.GenerateDynamic(ctx); err != nil {
				return summary, err
			}
			slog.Debug("Generated all Prometheus servers", slog.Int("count", gen.Count()))
		} else {
			slog.Info("No rules found, skipping Prometheus discovery")
		}
	}

	checkIterationChecks.Set(0)
	checkIterationChecksDone.Set(0)

	start := time.Now()
	defer func() {
		lastRunDuration.Set(time.Since(start).Seconds())
	}()

	jobs := make(chan scanJob, workers*5)
	results := make(chan reporter.Report, workers*5)
	wg := sync.WaitGroup{}

	ctx = context.WithValue(ctx, promapi.AllPrometheusServers, gen.Servers())
	for _, s := range cfg.Check {
		settings, _ := s.Decode()
		key := checks.SettingsKey(s.Name)
		ctx = context.WithValue(ctx, key, settings)
	}

	for w := 1; w <= workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			scanWorker(ctx, jobs, results)
		}()
	}

	go func() {
		defer close(results)
		wg.Wait()
	}()

	var onlineChecksCount, offlineChecksCount, checkedEntriesCount atomic.Int64
	go func() {
		for _, entry := range entries {
			switch {
			case entry.State == discovery.Excluded:
				continue
			case entry.PathError != nil && entry.State == discovery.Removed:
				continue
			case entry.Rule.Error.Err != nil && entry.State == discovery.Removed:
				continue
			case entry.PathError == nil && entry.Rule.Error.Err == nil:
				if entry.Rule.RecordingRule != nil {
					rulesParsedTotal.WithLabelValues(config.RecordingRuleType).Inc()
					slog.Debug("Found recording rule",
						slog.String("path", entry.SourcePath),
						slog.String("record", entry.Rule.RecordingRule.Record.Value.Value),
						slog.String("lines", entry.Rule.Lines.String()),
					)
				}
				if entry.Rule.AlertingRule != nil {
					rulesParsedTotal.WithLabelValues(config.AlertingRuleType).Inc()
					slog.Debug("Found alerting rule",
						slog.String("path", entry.SourcePath),
						slog.String("alert", entry.Rule.AlertingRule.Alert.Value.Value),
						slog.String("lines", entry.Rule.Lines.String()),
					)
				}

				checkedEntriesCount.Inc()
				checkList := cfg.GetChecksForRule(ctx, gen, entry, entry.DisabledChecks)
				for _, check := range checkList {
					checkIterationChecks.Inc()
					check := check
					if check.Meta().IsOnline {
						onlineChecksCount.Inc()
					} else {
						offlineChecksCount.Inc()
					}
					jobs <- scanJob{entry: entry, allEntries: entries, check: check}
				}
			default:
				if entry.Rule.Error.Err != nil {
					slog.Debug("Found invalid rule",
						slog.String("path", entry.SourcePath),
						slog.String("lines", entry.Rule.Lines.String()),
					)
					rulesParsedTotal.WithLabelValues(config.InvalidRuleType).Inc()
				}
				jobs <- scanJob{entry: entry, allEntries: entries, check: nil}
			}
		}
		defer close(jobs)
	}()

	for result := range results {
		summary.Report(result)
	}
	summary.SortReports()
	summary.Duration = time.Since(start)
	summary.TotalEntries = len(entries)
	summary.CheckedEntries = checkedEntriesCount.Load()
	summary.OnlineChecks = onlineChecksCount.Load()
	summary.OfflineChecks = offlineChecksCount.Load()

	lastRunTime.SetToCurrentTime()

	return summary, nil
}

type scanJob struct {
	allEntries []discovery.Entry
	entry      discovery.Entry
	check      checks.RuleChecker
}

func scanWorker(ctx context.Context, jobs <-chan scanJob, results chan<- reporter.Report) {
	for job := range jobs {
		job := job

		select {
		case <-ctx.Done():
			return
		default:
			var commentErr comments.CommentError
			var ignoreErr discovery.FileIgnoreError
			switch {
			case errors.As(job.entry.PathError, &ignoreErr):
				results <- reporter.Report{
					ReportedPath:  job.entry.ReportedPath,
					SourcePath:    job.entry.SourcePath,
					ModifiedLines: job.entry.ModifiedLines,
					Problem: checks.Problem{
						Lines:    []int{ignoreErr.Line},
						Reporter: ignoreFileReporter,
						Text:     ignoreErr.Error(),
						Severity: checks.Information,
					},
					Owner: job.entry.Owner,
				}
			case errors.As(job.entry.PathError, &commentErr):
				results <- reporter.Report{
					ReportedPath:  job.entry.ReportedPath,
					SourcePath:    job.entry.SourcePath,
					ModifiedLines: job.entry.ModifiedLines,
					Problem: checks.Problem{
						Lines:    []int{commentErr.Line},
						Reporter: pintCommentReporter,
						Text:     fmt.Sprintf("This comment is not a valid pint control comment: %s", commentErr.Error()),
						Severity: checks.Warning,
					},
					Owner: job.entry.Owner,
				}
			case job.entry.PathError != nil:
				line, e := tryDecodingYamlError(job.entry.PathError)
				results <- reporter.Report{
					ReportedPath:  job.entry.ReportedPath,
					SourcePath:    job.entry.SourcePath,
					ModifiedLines: job.entry.ModifiedLines,
					Problem: checks.Problem{
						Lines:    []int{line},
						Reporter: yamlParseReporter,
						Text:     fmt.Sprintf("YAML parser returned an error when reading this file: `%s`.", e),
						Details: `pint cannot read this file because YAML parser returned an error.
This usually means that you have an indention error or the file doesn't have the YAML structure required by Prometheus for [recording](https://prometheus.io/docs/prometheus/latest/configuration/recording_rules/) and [alerting](https://prometheus.io/docs/prometheus/latest/configuration/alerting_rules/) rules.
If this file is a template that will be rendered into valid YAML then you can instruct pint to ignore some lines using comments, see [pint docs](https://cloudflare.github.io/pint/ignoring.html).
`,
						Severity: checks.Fatal,
					},
					Owner: job.entry.Owner,
				}
			case job.entry.Rule.Error.Err != nil:
				results <- reporter.Report{
					ReportedPath:  job.entry.ReportedPath,
					SourcePath:    job.entry.SourcePath,
					ModifiedLines: job.entry.ModifiedLines,
					Rule:          job.entry.Rule,
					Problem: checks.Problem{
						Lines:    []int{job.entry.Rule.Error.Line},
						Reporter: yamlParseReporter,
						Text:     fmt.Sprintf("This rule is not a valid Prometheus rule: `%s`.", job.entry.Rule.Error.Err.Error()),
						Details: `This Prometheus rule is not valid.
This usually means that it's missing some required fields.`,
						Severity: checks.Fatal,
					},
					Owner: job.entry.Owner,
				}
			default:
				if job.entry.State == discovery.Unknown {
					slog.Warn(
						"Bug: unknown rule state",
						slog.String("path", job.entry.ReportedPath),
						slog.Int("line", job.entry.Rule.Lines.First),
						slog.String("name", job.entry.Rule.Name()),
					)
				}

				start := time.Now()
				problems := job.check.Check(ctx, job.entry.ReportedPath, job.entry.Rule, job.allEntries)
				checkDuration.WithLabelValues(job.check.Reporter()).Observe(time.Since(start).Seconds())
				for _, problem := range problems {
					results <- reporter.Report{
						ReportedPath:  job.entry.ReportedPath,
						SourcePath:    job.entry.SourcePath,
						ModifiedLines: job.entry.ModifiedLines,
						Rule:          job.entry.Rule,
						Problem:       problem,
						Owner:         job.entry.Owner,
					}
				}
			}
		}

		checkIterationChecksDone.Inc()
	}
}

func submitReports(reps []reporter.Reporter, summary reporter.Summary) (err error) {
	for _, rep := range reps {
		err = rep.Submit(summary)
		if err != nil {
			return err
		}
	}
	return nil
}
