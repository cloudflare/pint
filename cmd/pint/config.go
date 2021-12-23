package main

import (
	"fmt"
	"os"

	"github.com/cloudflare/pint/internal/config"

	"github.com/urfave/cli/v2"
)

func actionConfig(c *cli.Context) (err error) {
	err = initLogger(c.String(logLevelFlag), c.Bool(noColorFlag))
	if err != nil {
		return fmt.Errorf("failed to set log level: %w", err)
	}

	cfg, err := config.Load(c.Path(configFlag), c.IsSet(configFlag))
	if err != nil {
		return fmt.Errorf("failed to load config file %q: %w", c.Path(configFlag), err)
	}

	fmt.Fprintln(os.Stderr, cfg.String())

	return nil
}
