package main

import (
	"context"
	"fmt"
	"os"

	"github.com/cloudflare/pint/internal/config"

	"github.com/urfave/cli/v3"
)

var configCmd = &cli.Command{
	Name:   "config",
	Usage:  "Parse and print used config.",
	Action: actionConfig,
}

func actionConfig(_ context.Context, c *cli.Command) (err error) {
	err = initLogger(c.String(logLevelFlag), c.Bool(noColorFlag))
	if err != nil {
		return fmt.Errorf("failed to set log level: %w", err)
	}

	cfg, _, err := config.Load(c.String(configFlag), c.IsSet(configFlag))
	if err != nil {
		return fmt.Errorf("failed to load config file %q: %w", c.String(configFlag), err)
	}

	fmt.Fprintln(os.Stderr, cfg.String())

	return nil
}
