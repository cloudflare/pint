package main

import (
	"fmt"

	"github.com/urfave/cli/v2"
)

var versionCmd = &cli.Command{
	Name:   "version",
	Usage:  "Print version and exit",
	Action: actionVersion,
}

func actionVersion(c *cli.Context) (err error) {
	err = initLogger(c.String(logLevelFlag), c.Bool(noColorFlag))
	if err != nil {
		return fmt.Errorf("failed to set log level: %w", err)
	}
	fmt.Printf("%s (revision: %s)\n", version, commit)
	return nil
}
