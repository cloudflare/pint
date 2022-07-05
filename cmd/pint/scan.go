package main

import (
	"context"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

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

const yamlParseReporter = "yaml/parse"

func tryDecodingYamlError(e string) (int, string) {
	for _, re := range []*regexp.Regexp{yamlErrRe, yamlUnmarshalErrRe, rulefmtGroupRe, rulefmtGroupnameRe} {
		parts := re.FindStringSubmatch(e)
		if len(parts) > 2 {
			line, err := strconv.Atoi(parts[1])
			if err != nil {
				return 1, e
			}
			return line, parts[2]
		}
	}
	return 1, e
}

func checkRules(ctx context.Context, workers int, cfg config.Config, entries []discovery.Entry) (summary reporter.Summary) {
	checkIterationChecks.Set(float64(len(entries)))
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

	go func() {
		for _, entry := range entries {
			if entry.PathError == nil && entry.Rule.Error.Err == nil {
				if entry.Rule.RecordingRule != nil {
					rulesParsedTotal.WithLabelValues(config.RecordingRuleType).Inc()
					log.Debug().
						Str("path", entry.Path).
						Str("record", entry.Rule.RecordingRule.Record.Value.Value).
						Str("lines", output.FormatLineRangeString(entry.Rule.Lines())).
						Msg("Found recording rule")
				}
				if entry.Rule.AlertingRule != nil {
					rulesParsedTotal.WithLabelValues(config.AlertingRuleType).Inc()
					log.Debug().
						Str("path", entry.Path).
						Str("alert", entry.Rule.AlertingRule.Alert.Value.Value).
						Str("lines", output.FormatLineRangeString(entry.Rule.Lines())).
						Msg("Found alerting rule")
				}

				checkList := cfg.GetChecksForRule(ctx, entry.Path, entry.Rule)
				for _, check := range checkList {
					check := check
					jobs <- scanJob{entry: entry, allEntries: entries, check: check}
				}
			} else {
				if entry.Rule.Error.Err != nil {
					log.Debug().
						Str("path", entry.Path).
						Str("lines", output.FormatLineRangeString(entry.Rule.Lines())).
						Msg("Found invalid rule")
					rulesParsedTotal.WithLabelValues(config.InvalidRuleType).Inc()
				}

				jobs <- scanJob{entry: entry, allEntries: entries, check: nil}
			}
			checkIterationChecksDone.Inc()
		}
		defer close(jobs)
	}()

	for result := range results {
		summary.Reports = append(summary.Reports, result)
	}

	lastRunTime.SetToCurrentTime()

	return
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
			if job.entry.PathError != nil {
				line, e := tryDecodingYamlError(job.entry.PathError.Error())
				results <- reporter.Report{
					Path:          job.entry.Path,
					ModifiedLines: job.entry.ModifiedLines,
					Problem: checks.Problem{
						Lines:    []int{line},
						Reporter: yamlParseReporter,
						Text:     e,
						Severity: checks.Fatal,
					},
					Owner: job.entry.Owner,
				}
			} else if job.entry.Rule.Error.Err != nil {
				results <- reporter.Report{
					Path:          job.entry.Path,
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
			} else {
				start := time.Now()
				problems := job.check.Check(ctx, job.entry.Rule, job.allEntries)
				duration := time.Since(start)
				checkDuration.WithLabelValues(job.check.Reporter()).Observe(duration.Seconds())
				for _, problem := range problems {
					results <- reporter.Report{
						Path:          job.entry.Path,
						ModifiedLines: job.entry.ModifiedLines,
						Rule:          job.entry.Rule,
						Problem:       problem,
						Owner:         job.entry.Owner,
					}
				}
			}
		}
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
