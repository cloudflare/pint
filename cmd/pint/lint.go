package main

import (
	"context"
	"fmt"
	"os"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/config"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/reporter"

	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"
)

var lintCmd = &cli.Command{
	Name:   "lint",
	Usage:  "Lint specified files",
	Action: actionLint,
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

	finder := discovery.NewGlobFinder(paths...)
	entries, err := finder.Find()
	if err != nil {
		return err
	}

	ctx := context.WithValue(context.Background(), config.CommandKey, config.LintCommand)
	summary := checkRules(ctx, meta.workers, meta.cfg, entries)

	r := reporter.NewConsoleReporter(os.Stderr)
	err = r.Submit(summary)
	if err != nil {
		return err
	}

	bySeverity := map[string]interface{}{}
	var problems int
	for s, c := range summary.CountBySeverity() {
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
