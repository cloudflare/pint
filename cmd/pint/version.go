package main

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"
)

var versionCmd = &cli.Command{
	Name:   "version",
	Usage:  "Print version and exit.",
	Action: actionVersion,
}

func actionVersion(_ context.Context, _ *cli.Command) error {
	fmt.Printf("%s (revision: %s)\n", version, commit)
	return nil
}
