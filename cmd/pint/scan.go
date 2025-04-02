package main

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/config"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/promapi"
	"github.com/cloudflare/pint/internal/reporter"
)

func checkRules(ctx context.Context, workers int, isOffline bool, gen *config.PrometheusGenerator, cfg config.Config, entries []discovery.Entry) (summary reporter.Summary, err error) {
	slog.Info("Checking Prometheus rules", slog.Int("entries", len(entries)), slog.Int("workers", workers), slog.Bool("online", !isOffline))
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
			case entry.PathError != nil && entry.State == discovery.Removed:
				continue
			case entry.Rule.Error.Err != nil && entry.State == discovery.Removed:
				continue
			default:
				if entry.Rule.RecordingRule != nil {
					rulesParsedTotal.WithLabelValues(config.RecordingRuleType).Inc()
					slog.Debug("Found recording rule",
						slog.String("path", entry.Path.Name),
						slog.String("record", entry.Rule.RecordingRule.Record.Value),
						slog.String("lines", entry.Rule.Lines.String()),
						slog.String("state", entry.State.String()),
					)
				}
				if entry.Rule.AlertingRule != nil {
					rulesParsedTotal.WithLabelValues(config.AlertingRuleType).Inc()
					slog.Debug("Found alerting rule",
						slog.String("path", entry.Path.Name),
						slog.String("alert", entry.Rule.AlertingRule.Alert.Value),
						slog.String("lines", entry.Rule.Lines.String()),
						slog.String("state", entry.State.String()),
					)
				}
				if entry.Rule.Error.Err != nil {
					slog.Debug("Found invalid rule",
						slog.String("path", entry.Path.Name),
						slog.String("lines", entry.Rule.Lines.String()),
						slog.String("state", entry.State.String()),
					)
					rulesParsedTotal.WithLabelValues(config.InvalidRuleType).Inc()
				}

				checkedEntriesCount.Add(1)
				checkList := cfg.GetChecksForEntry(ctx, gen, entry)
				for _, check := range checkList {
					checkIterationChecks.Inc()
					if check.Meta().Online {
						onlineChecksCount.Add(1)
					} else {
						offlineChecksCount.Add(1)
					}
					jobs <- scanJob{entry: entry, allEntries: entries, check: check}
				}
			}
		}
		defer close(jobs)
	}()

	for result := range results {
		summary.Report(result)
	}
	summary.Duration = time.Since(start)
	summary.TotalEntries = len(entries)
	summary.CheckedEntries = checkedEntriesCount.Load()
	summary.OnlineChecks = onlineChecksCount.Load()
	summary.OfflineChecks = offlineChecksCount.Load()

	for _, prom := range gen.Servers() {
		for api, names := range prom.GetDisabledChecks() {
			summary.MarkCheckDisabled(prom.Name(), api, names)
		}
	}
	for _, pd := range summary.GetPrometheusDetails() {
		for _, dc := range pd.DisabledChecks {
			slog.Warn(
				"Some checks were disabled because configured server doesn't seem to support all Prometheus APIs",
				slog.String("prometheus", pd.Name),
				slog.String("api", dc.API),
				slog.Any("checks", dc.Checks),
			)
		}
	}

	lastRunTime.SetToCurrentTime()

	return summary, nil
}

type scanJob struct {
	check      checks.RuleChecker
	allEntries []discovery.Entry
	entry      discovery.Entry
}

func scanWorker(ctx context.Context, jobs <-chan scanJob, results chan<- reporter.Report) {
	for job := range jobs {
		select {
		case <-ctx.Done():
			return
		default:
			if job.entry.State == discovery.Unknown {
				slog.Warn(
					"Bug: unknown rule state",
					slog.String("path", job.entry.Path.String()),
					slog.Int("line", job.entry.Rule.Lines.First),
					slog.String("name", job.entry.Rule.Name()),
				)
			}

			start := time.Now()
			problems := job.check.Check(ctx, job.entry.Path, job.entry.Rule, job.allEntries)
			checkDuration.WithLabelValues(job.check.Reporter()).Observe(time.Since(start).Seconds())
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

		checkIterationChecksDone.Inc()
	}
}
