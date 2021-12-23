package main

import (
	"fmt"
	"os"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/config"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/reporter"

	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"
)

func actionLint(c *cli.Context) (err error) {
	err = initLogger(c.String(logLevelFlag), c.Bool(noColorFlag))
	if err != nil {
		return fmt.Errorf("failed to set log level: %w", err)
	}

	paths := c.Args().Slice()
	if len(paths) == 0 {
		return fmt.Errorf("at least one file or directory required")
	}

	cfg, err := config.Load(c.Path(configFlag), c.IsSet(configFlag))
	if err != nil {
		return fmt.Errorf("failed to load config file %q: %w", c.Path(configFlag), err)
	}
	cfg.SetDisabledChecks(c.Bool(offlineFlag), c.StringSlice(disabledFlag))

	d := discovery.NewGlobFileFinder()
	toScan, err := d.Find(paths...)
	if err != nil {
		return err
	}

	if len(toScan.Paths()) == 0 {
		return fmt.Errorf("no matching files")
	}

	summary := scanFiles(cfg, toScan, &discovery.NoopLineFinder{})

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
	if problems > 0 {
		log.Info().Fields(bySeverity).Msg("Problems found")
		return fmt.Errorf("problems found")
	}

	return nil
}
