package main

import (
	"context"
	"errors"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/prometheus/model/rulefmt"
	"github.com/rs/zerolog/log"
	"go.uber.org/atomic"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/config"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/output"
	"github.com/cloudflare/pint/internal/reporter"
)

var (
	yamlErrRe          = regexp.MustCompile("^yaml: line (.+): (.+)")
	yamlUnmarshalErrRe = regexp.MustCompile("^yaml: unmarshal errors:\n  line (.+): (.+)")
	rulefmtGroupRe     = regexp.MustCompile("^([0-9]+):[0-9]+: group \".+\", rule [0-9]+, (.+)")
	rulefmtGroupnameRe = regexp.MustCompile("^([0-9]+):[0-9]+: (groupname: .+)")
)

const (
	yamlParseReporter  = "yaml/parse"
	ignoreFileReporter = "ignore/file"
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

func checkRules(ctx context.Context, workers int, cfg config.Config, entries []discovery.Entry) (summary reporter.Summary) {
	checkIterationChecks.Set(0)
	checkIterationChecksDone.Set(0)

	start := time.Now()
	defer func() {
		lastRunDuration.Set(time.Since(start).Seconds())
	}()

	jobs := make(chan scanJob, workers*5)
	results := make(chan reporter.Report, workers*5)
	wg := sync.WaitGroup{}

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

	var onlineChecksCount, offlineChecksCount atomic.Int64
	go func() {
		for _, entry := range entries {
			switch {
			case entry.PathError == nil && entry.Rule.Error.Err == nil:
				if entry.Rule.RecordingRule != nil {
					rulesParsedTotal.WithLabelValues(config.RecordingRuleType).Inc()
					log.Debug().
						Str("path", entry.SourcePath).
						Str("record", entry.Rule.RecordingRule.Record.Value.Value).
						Str("lines", output.FormatLineRangeString(entry.Rule.Lines())).
						Msg("Found recording rule")
				}
				if entry.Rule.AlertingRule != nil {
					rulesParsedTotal.WithLabelValues(config.AlertingRuleType).Inc()
					log.Debug().
						Str("path", entry.SourcePath).
						Str("alert", entry.Rule.AlertingRule.Alert.Value.Value).
						Str("lines", output.FormatLineRangeString(entry.Rule.Lines())).
						Msg("Found alerting rule")
				}

				checkList := cfg.GetChecksForRule(ctx, entry.SourcePath, entry.Rule)
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
					log.Debug().
						Str("path", entry.SourcePath).
						Str("lines", output.FormatLineRangeString(entry.Rule.Lines())).
						Msg("Found invalid rule")
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
	summary.Duration = time.Since(start)
	summary.Entries = len(entries)
	summary.OnlineChecks = onlineChecksCount.Load()
	summary.OfflineChecks = offlineChecksCount.Load()

	lastRunTime.SetToCurrentTime()

	return summary
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
			switch {
			case errors.Is(job.entry.PathError, discovery.ErrFileIsIgnored):
				results <- reporter.Report{
					ReportedPath:  job.entry.ReportedPath,
					SourcePath:    job.entry.SourcePath,
					ModifiedLines: job.entry.ModifiedLines,
					Problem: checks.Problem{
						Lines:    job.entry.ModifiedLines,
						Reporter: ignoreFileReporter,
						Text:     "This file was excluded from pint checks",
						Severity: checks.Information,
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
						Text:     e,
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
						Fragment: job.entry.Rule.Error.Fragment,
						Lines:    []int{job.entry.Rule.Error.Line},
						Reporter: yamlParseReporter,
						Text:     job.entry.Rule.Error.Err.Error(),
						Severity: checks.Fatal,
					},
					Owner: job.entry.Owner,
				}
			default:
				start := time.Now()
				problems := job.check.Check(ctx, job.entry.Rule, job.allEntries)
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
