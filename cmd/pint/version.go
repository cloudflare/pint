package main

import (
	"fmt"

	"github.com/urfave/cli/v2"
)

var versionCmd = &cli.Command{
	Name:   "version",
	Usage:  "Print version and exit.",
	Action: actionVersion,
}

func actionVersion(_ *cli.Context) error {
	fmt.Printf("%s (revision: %s)\n", version, commit)
	return nil
}
