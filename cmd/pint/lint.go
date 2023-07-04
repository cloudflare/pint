package main

import (
	"context"
	"fmt"
	"os"
	"regexp"

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
		&cli.StringFlag{
			Name:    failOnFlag,
			Aliases: []string{"w"},
			Value:   "bug",
			Usage:   "Exit with non-zero code if there are problems with given severity (or higher) detected",
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
		prom.StartWorkers()
	}
	defer meta.cleanup()

	ctx := context.WithValue(context.Background(), config.CommandKey, config.LintCommand)
	summary := checkRules(ctx, meta.workers, meta.cfg, entries)

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

	r := reporter.NewConsoleReporter(os.Stderr, minSeverity)
	err = r.Submit(summary)
	if err != nil {
		return err
	}

	err = report(summary, meta.cfg.Reporters)
	if err != nil {
		return err
	}

	bySeverity := map[string]interface{}{} // interface{} is needed for log.Fields()
	var problems, hiddenProblems, failProblems int
	for s, c := range summary.CountBySeverity() {
		if s >= failOn {
			failProblems++
		}
		if s < minSeverity {
			hiddenProblems++
		}
		bySeverity[s.String()] = c
		if s >= checks.Bug {
			problems += c
		}
	}
	if len(bySeverity) > 0 {
		log.Info().Fields(bySeverity).Msg("Problems found")
	}
	if hiddenProblems > 0 {
		log.Info().Msgf("%d problem(s) not visible because of --%s=%s flag", hiddenProblems, minSeverityFlag, c.String(minSeverityFlag))
	}

	if failProblems > 0 {
		return fmt.Errorf("found %d problem(s) with severity %s or higher", failProblems, failOn)
	}
	return nil
}

func report(summary reporter.Summary, reporters *config.Reporters) error {

	if reporters.JSON != nil {
		r := reporter.NewJSONReporter(reporters.JSON.Path)
		err := r.Submit(summary.Reports())
		if err != nil {
			return err
		}
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
			goto NEXT
		}
		for _, re := range allowedOwners {
			if re.MatchString(entry.Owner) {
				goto NEXT
			}
		}
		reports = append(reports, reporter.Report{
			ReportedPath:  entry.ReportedPath,
			SourcePath:    entry.SourcePath,
			ModifiedLines: entry.ModifiedLines,
			Rule:          entry.Rule,
			Problem: checks.Problem{
				Lines:    entry.Rule.Lines(),
				Reporter: discovery.RuleOwnerComment,
				Text:     fmt.Sprintf("this rule is set as owned by %q but %q doesn't match any of the allowed owner values", entry.Owner, entry.Owner),
				Severity: checks.Bug,
			},
		})
	NEXT:
	}
	return reports
}
