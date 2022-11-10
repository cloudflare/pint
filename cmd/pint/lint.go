package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/config"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/reporter"

	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"
)

var requireOwnerFlag = "require-owner"

var lintCmd = &cli.Command{
	Name:   "lint",
	Usage:  "Lint specified files",
	Action: actionLint,
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    requireOwnerFlag,
			Aliases: []string{"r"},
			Value:   false,
			Usage:   "Require all rules to have an owner set via comment",
		},
		&cli.StringFlag{
			Name:    minSeverityFlag,
			Aliases: []string{"n"},
			Value:   "warning",
			Usage:   "Set minimum severity for reported problems",
		},
	},
}

func actionLint(c *cli.Context) error {
	meta, err := actionSetup(c)
	if err != nil {
		return err
	}

	paths := c.Args().Slice()
	if len(paths) == 0 {
		return fmt.Errorf("at least one file or directory required")
	}

	finder := discovery.NewGlobFinder(paths, meta.cfg.Parser.CompileRelaxed())
	entries, err := finder.Find()
	if err != nil {
		return err
	}

	for _, prom := range meta.cfg.PrometheusServers {
		prom.StartWorkers(time.Hour)
	}
	defer meta.cleanup()

	ctx := context.WithValue(context.Background(), config.CommandKey, config.LintCommand)
	summary := checkRules(ctx, meta.workers, meta.cfg, entries)

	if c.Bool(requireOwnerFlag) {
		summary.Report(verifyOwners(entries)...)
	}

	minSeverity, err := checks.ParseSeverity(c.String(minSeverityFlag))
	if err != nil {
		return fmt.Errorf("invalid %s value: %w", minSeverityFlag, err)
	}

	r := reporter.NewConsoleReporter(os.Stderr, minSeverity)
	err = r.Submit(summary)
	if err != nil {
		return err
	}

	bySeverity := map[string]interface{}{} // interface{} is needed for log.Fields()
	var problems int
	for s, c := range summary.CountBySeverity() {
		if s < minSeverity {
			continue
		}
		bySeverity[s.String()] = c
		if s >= checks.Bug {
			problems += c
		}
	}
	if len(bySeverity) > 0 {
		log.Info().Fields(bySeverity).Msg("Problems found")
	}
	if problems > 0 {
		return fmt.Errorf("problems found")
	}

	return nil
}

func verifyOwners(entries []discovery.Entry) (reports []reporter.Report) {
	for _, entry := range entries {
		if entry.PathError != nil {
			continue
		}
		if entry.Owner != "" {
			continue
		}
		reports = append(reports, reporter.Report{
			ReportedPath:  entry.ReportedPath,
			SourcePath:    entry.SourcePath,
			ModifiedLines: entry.ModifiedLines,
			Rule:          entry.Rule,
			Problem: checks.Problem{
				Lines:    entry.Rule.Lines(),
				Reporter: discovery.RuleOwnerComment,
				Text: fmt.Sprintf(`%s comments are required in all files, please add a "# pint %s $owner" somewhere in this file and/or "# pint %s $owner" on top of each rule`,
					discovery.RuleOwnerComment, discovery.FileOwnerComment, discovery.RuleOwnerComment),
				Severity: checks.Bug,
			},
		})
	}
	return
}
