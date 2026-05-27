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

func checkRules(ctx context.Context, workers int, isOffline bool, gen *config.PrometheusGenerator, cfg config.Config, entries []*discovery.Entry) (summary reporter.Summary, err error) {
	slog.LogAttrs(ctx, slog.LevelInfo, "Checking Prometheus rules", slog.Int("entries", len(entries)), slog.Int("workers", workers), slog.Bool("online", !isOffline))
	if isOffline {
		slog.LogAttrs(ctx, slog.LevelInfo, "Offline mode, skipping Prometheus discovery")
	} else {
		if len(entries) > 0 {
			if err = gen.GenerateDynamic(ctx); err != nil {
				return summary, err
			}
			slog.LogAttrs(ctx, slog.LevelDebug, "Generated all Prometheus servers", slog.Int("count", gen.Count()))
		} else {
			slog.LogAttrs(ctx, slog.LevelInfo, "No rules found, skipping Prometheus discovery")
		}
	}

	checkIterationChecks.Set(0)
	checkIterationChecksDone.Set(0)

	start := time.Now()
	defer func() {
		lastRunDuration.Set(time.Since(start).Seconds())
	}()

	concurrencyLimit := make(chan struct{}, workers)
	wg := sync.WaitGroup{}
	var mu sync.Mutex
	var reports []reporter.Report

	ctx = context.WithValue(ctx, promapi.AllPrometheusServers, gen.Servers())
	for _, s := range cfg.Check {
		settings, _ := s.Decode()
		key := checks.SettingsKey(s.Name)
		ctx = context.WithValue(ctx, key, settings)
	}

	var onlineChecksCount, offlineChecksCount, checkedEntriesCount atomic.Int64
	for _, entry := range entries {
		switch {
		case entry.PathError != nil && entry.State == discovery.Removed:
			continue
		case entry.Rule.Error.Err != nil && entry.State == discovery.Removed:
			continue
		default:
			if entry.Rule.RecordingRule != nil {
				rulesParsedTotal.WithLabelValues(config.RecordingRuleType).Inc()
				slog.LogAttrs(
					ctx, slog.LevelDebug, "Found recording rule",
					slog.String("path", entry.Path.Name),
					slog.String("record", entry.Rule.RecordingRule.Record.Value),
					slog.String("lines", entry.Rule.Lines.String()),
					slog.String("state", entry.State.String()),
				)
			}
			if entry.Rule.AlertingRule != nil {
				rulesParsedTotal.WithLabelValues(config.AlertingRuleType).Inc()
				slog.LogAttrs(
					ctx, slog.LevelDebug, "Found alerting rule",
					slog.String("path", entry.Path.Name),
					slog.String("alert", entry.Rule.AlertingRule.Alert.Value),
					slog.String("lines", entry.Rule.Lines.String()),
					slog.String("state", entry.State.String()),
				)
			}
			if entry.Rule.Error.Err != nil {
				slog.LogAttrs(
					ctx, slog.LevelDebug, "Found invalid rule",
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
				concurrencyLimit <- struct{}{}
				wg.Go(func() {
					defer func() { <-concurrencyLimit }()
					results := runCheck(ctx, check, entry, entries)
					if len(results) > 0 {
						mu.Lock()
						reports = append(reports, results...)
						mu.Unlock()
					}
				})
			}
		}
	}
	wg.Wait()

	for _, result := range reports {
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
			slog.LogAttrs(
				ctx, slog.LevelWarn,
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

func runCheck(ctx context.Context, check checks.RuleChecker, entry *discovery.Entry, entries []*discovery.Entry) []reporter.Report {
	select {
	case <-ctx.Done():
		return nil
	default:
	}

	if entry.State == discovery.Unknown {
		slog.LogAttrs(
			ctx, slog.LevelWarn,
			"Bug: unknown rule state",
			slog.String("path", entry.Path.String()),
			slog.Int("line", entry.Rule.Lines.First),
			slog.String("name", entry.Rule.Name()),
		)
	}

	start := time.Now()
	problems := check.Check(ctx, entry, entries)
	checkDuration.WithLabelValues(check.Reporter()).Observe(time.Since(start).Seconds())

	var reports []reporter.Report
	for _, problem := range problems {
		reports = append(reports, reporter.Report{
			Path:        entry.Path,
			Changes:     entry.Changes,
			Rule:        entry.Rule,
			Problem:     problem,
			Owner:       entry.Owner,
			IsDuplicate: false,
			Duplicates:  nil,
		})
	}

	checkIterationChecksDone.Inc()
	return reports
}
