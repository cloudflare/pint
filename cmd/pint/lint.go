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
	Flags: []cli.Flag{
		&cli.StringSliceFlag{
			Name:    disabledFlag,
			Aliases: []string{"d"},
			Value:   cli.NewStringSlice(),
			Usage:   "List of checks to disable (example: promql/cost)",
		},
		&cli.BoolFlag{
			Name:    offlineFlag,
			Aliases: []string{"o"},
			Value:   false,
			Usage:   "Disable all check that send live queries to Prometheus servers",
		},
	},
}

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
	cfg.SetDisabledChecks(c.StringSlice(disabledFlag))
	if c.Bool(offlineFlag) {
		cfg.DisableOnlineChecks()
	}

	d := discovery.NewGlobFileFinder()
	toScan, err := d.Find(paths...)
	if err != nil {
		return err
	}

	if len(toScan.Paths()) == 0 {
		return fmt.Errorf("no matching files")
	}

	summary := scanFiles(context.Background(), cfg, toScan, &discovery.NoopLineFinder{})

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
