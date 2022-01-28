package main

import (
	"context"
	"os"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/config"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/output"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/reporter"

	"github.com/rs/zerolog/log"
)

var yamlErrRe = regexp.MustCompile("^yaml: line (.+): (.+)$")

func tryDecodingYamlError(e string) (int, string) {
	parts := yamlErrRe.FindStringSubmatch(e)
	if len(parts) > 2 {
		line, err := strconv.Atoi(parts[1])
		if err != nil {
			return 1, e
		}
		return line, parts[2]
	}
	return 1, e
}

func scanProblem(path string, err error) reporter.Report {
	line, e := tryDecodingYamlError(err.Error())
	return reporter.Report{
		Path: path,
		Problem: checks.Problem{
			Lines:    []int{line},
			Reporter: "pint/parse",
			Text:     e,
			Severity: checks.Fatal,
		},
	}
}

func scanFiles(ctx context.Context, workers int, cfg config.Config, fcs discovery.FileFindResults, ld discovery.LineFinder) (summary reporter.Summary) {
	summary.FileChanges = fcs

	scanJobs := []scanJob{}

	p := parser.NewParser()

	for _, path := range summary.FileChanges.Paths() {
		path := path

		lineResults, err := ld.Find(path)
		if err != nil {
			summary.Reports = append(summary.Reports, scanProblem(path, err))
			log.Error().Str("path", path).Err(err).Msg("Failed to discover line numbers")
			continue
		}

		f, err := os.Open(path)
		if err != nil {
			summary.Reports = append(summary.Reports, scanProblem(path, err))
			log.Error().Str("path", path).Err(err).Msg("Failed to open file for reading")
			continue
		}

		content, err := parser.ReadContent(f)
		f.Close()
		if err != nil {
			summary.Reports = append(summary.Reports, scanProblem(path, err))
			log.Error().Str("path", path).Err(err).Msg("Failed to read file content")
			continue
		}

		rules, err := p.Parse(content)
		if err != nil {
			summary.Reports = append(summary.Reports, scanProblem(path, err))
			log.Error().Str("path", path).Err(err).Msg("Failed to parse file content")
			continue
		}
		log.Info().Str("path", path).Int("rules", len(rules)).Msg("File parsed")

		for _, rule := range rules {
			rule := rule

			if rule.AlertingRule != nil {
				log.Debug().
					Str("path", path).
					Str("alert", rule.AlertingRule.Alert.Value.Value).
					Str("lines", output.FormatLineRangeString(rule.Lines())).
					Msg("Found alerting rule")
			} else if rule.RecordingRule != nil {
				log.Debug().
					Str("path", path).
					Str("record", rule.RecordingRule.Record.Value.Value).
					Str("lines", output.FormatLineRangeString(rule.Lines())).
					Msg("Found recording rule")
			} else if rule.Error.Err != nil {
				log.Debug().
					Str("path", path).
					Str("lines", output.FormatLineRangeString(rule.Lines())).
					Msg("Found invalid rule")
			}

			if rule.Error.Err == nil {
				checkList := cfg.GetChecksForRule(ctx, path, rule)
				for _, check := range checkList {
					check := check
					scanJobs = append(scanJobs, scanJob{path: path, rule: rule, check: check, lines: lineResults})
				}
			} else {
				scanJobs = append(scanJobs, scanJob{path: path, rule: rule, check: nil, lines: lineResults})
			}
		}
	}

	jobs := make(chan scanJob, workers*5)
	results := make(chan reporter.Report, workers*5)
	wg := sync.WaitGroup{}

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
		for _, scanJob := range scanJobs {
			jobs <- scanJob
		}
		defer close(jobs)
	}()

	for result := range results {
		summary.Reports = append(summary.Reports, result)
	}
	return
}

type scanJob struct {
	path  string
	rule  parser.Rule
	check checks.RuleChecker
	lines discovery.LineFindResults
}

func scanWorker(ctx context.Context, jobs <-chan scanJob, results chan<- reporter.Report) {
	for job := range jobs {
		job := job

		select {
		case <-ctx.Done():
			return
		default:
			if job.rule.Error.Err != nil {
				results <- reporter.Report{Path: job.path, Rule: job.rule, Problem: checks.Problem{
					Fragment: job.rule.Error.Fragment,
					Lines:    []int{job.rule.Error.Line},
					Reporter: "pint/parse",
					Text:     job.rule.Error.Err.Error(),
					Severity: checks.Fatal,
				}}
			} else {
				start := time.Now()
				probles := job.check.Check(ctx, job.rule)
				duration := time.Since(start)
				checkDuration.WithLabelValues(job.check.Reporter()).Observe(duration.Seconds())
				for _, problem := range probles {
					if job.lines.HasLines(problem.Lines) {
						results <- reporter.Report{Path: job.path, Rule: job.rule, Problem: problem}
					} else {
						log.Debug().Str("path", job.path).Str("lines", output.FormatLineRangeString(problem.Lines)).Msg("Problem reported on unmodified lines, ignoring")
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
