package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"regexp"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/config"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/git"
	"github.com/cloudflare/pint/internal/reporter"

	"github.com/urfave/cli/v2"
)

var requireOwnerFlag = "require-owner"

var lintCmd = &cli.Command{
	Name:   "lint",
	Usage:  "Check specified files or directories (can be a glob).",
	Action: actionLint,
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    requireOwnerFlag,
			Aliases: []string{"r"},
			Value:   false,
			Usage:   "Require all rules to have an owner set via comment.",
		},
		&cli.StringFlag{
			Name:    minSeverityFlag,
			Aliases: []string{"n"},
			Value:   "warning",
			Usage:   "Set minimum severity for reported problems.",
		},
		&cli.StringFlag{
			Name:    failOnFlag,
			Aliases: []string{"w"},
			Value:   "bug",
			Usage:   "Exit with non-zero code if there are problems with given severity (or higher) detected.",
		},
		&cli.BoolFlag{
			Name:    teamCityFlag,
			Aliases: []string{"t"},
			Value:   false,
			Usage:   "Report problems using TeamCity Service Messages.",
		},
		&cli.BoolFlag{
			Name:    checkStyleFlag,
			Aliases: []string{"c"},
			Value:   false,
			Usage:   "Report problems using checkstyle xml.",
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
		return errors.New("at least one file or directory required")
	}

	slog.Info("Finding all rules to check", slog.Any("paths", paths))
	finder := discovery.NewGlobFinder(paths, git.NewPathFilter(
		config.MustCompileRegexes(meta.cfg.Parser.Include...),
		config.MustCompileRegexes(meta.cfg.Parser.Exclude...),
		config.MustCompileRegexes(meta.cfg.Parser.Relaxed...),
	))
	entries, err := finder.Find()
	if err != nil {
		return err
	}

	ctx := context.WithValue(context.Background(), config.CommandKey, config.LintCommand)

	gen := config.NewPrometheusGenerator(meta.cfg, metricsRegistry)
	defer gen.Stop()

	if err = gen.GenerateStatic(); err != nil {
		return err
	}

	summary, err := checkRules(ctx, meta.workers, meta.isOffline, gen, meta.cfg, entries)
	if err != nil {
		return err
	}

	if c.Bool(requireOwnerFlag) {
		summary.Report(verifyOwners(entries, meta.cfg.Owners.CompileAllowed())...)
	}

	minSeverity, err := checks.ParseSeverity(c.String(minSeverityFlag))
	if err != nil {
		return fmt.Errorf("invalid --%s value: %w", minSeverityFlag, err)
	}
	failOn, err := checks.ParseSeverity(c.String(failOnFlag))
	if err != nil {
		return fmt.Errorf("invalid --%s value: %w", failOnFlag, err)
	}

	var r reporter.Reporter
	if c.Bool(checkStyleFlag) {
		r = reporter.NewCheckStyleReporter(os.Stdout)
	}
	if c.Bool(teamCityFlag) {
		r = reporter.NewTeamCityReporter(os.Stderr)
	} else {
		r = reporter.NewConsoleReporter(os.Stderr, minSeverity)
	}

	err = r.Submit(summary)
	if err != nil {
		return err
	}

	bySeverity := summary.CountBySeverity()
	var problems, hiddenProblems, failProblems int
	for s, c := range bySeverity {
		if s >= failOn {
			failProblems++
		}
		if s < minSeverity {
			hiddenProblems++
		}
		if s >= checks.Bug {
			problems += c
		}
	}
	if len(bySeverity) > 0 {
		slog.Info("Problems found", logSeverityCounters(bySeverity)...)
	}
	if hiddenProblems > 0 {
		slog.Info(fmt.Sprintf("%d problem(s) not visible because of --%s=%s flag", hiddenProblems, minSeverityFlag, c.String(minSeverityFlag)))
	}

	if failProblems > 0 {
		return fmt.Errorf("found %d problem(s) with severity %s or higher", failProblems, failOn)
	}
	return nil
}

func verifyOwners(entries []discovery.Entry, allowedOwners []*regexp.Regexp) (reports []reporter.Report) {
	for _, entry := range entries {
		if entry.State == discovery.Removed {
			continue
		}
		if entry.PathError != nil {
			continue
		}
		if entry.Owner == "" {
			reports = append(reports, reporter.Report{
				Path: discovery.Path{
					Name:          entry.Path.Name,
					SymlinkTarget: entry.Path.SymlinkTarget,
				},
				ModifiedLines: entry.ModifiedLines,
				Rule:          entry.Rule,
				Problem: checks.Problem{
					Lines:    entry.Rule.Lines,
					Reporter: discovery.RuleOwnerComment,
					Text: fmt.Sprintf("`%s` comments are required in all files, please add a `# pint %s $owner` somewhere in this file and/or `# pint %s $owner` on top of each rule.",
						discovery.RuleOwnerComment, discovery.FileOwnerComment, discovery.RuleOwnerComment),
					Severity: checks.Bug,
				},
			})
			goto NEXT
		}
		for _, re := range allowedOwners {
			if re.MatchString(entry.Owner) {
				goto NEXT
			}
		}
		reports = append(reports, reporter.Report{
			Path: discovery.Path{
				Name:          entry.Path.Name,
				SymlinkTarget: entry.Path.SymlinkTarget,
			},
			ModifiedLines: entry.ModifiedLines,
			Rule:          entry.Rule,
			Problem: checks.Problem{
				Lines:    entry.Rule.Lines,
				Reporter: discovery.RuleOwnerComment,
				Text:     fmt.Sprintf("This rule is set as owned by `%s` but `%s` doesn't match any of the allowed owner values.", entry.Owner, entry.Owner),
				Severity: checks.Bug,
			},
		})
	NEXT:
	}
	return reports
}
