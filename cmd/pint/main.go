package main

import (
	"fmt"
	"os"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"
)

const (
	configFlag   = "config"
	logLevelFlag = "log-level"
	disabledFlag = "disabled"
	offlineFlag  = "offline"
)

var (
	version = "unknown"
	commit  = "unknown"
)

func newApp() *cli.App {
	return &cli.App{
		Usage: "Prometheus rule linter",
		Flags: []cli.Flag{
			&cli.PathFlag{
				Name:    configFlag,
				Aliases: []string{"c"},
				Value:   ".pint.hcl",
				Usage:   "Configuration file to use",
			},
			&cli.StringFlag{
				Name:    logLevelFlag,
				Aliases: []string{"l"},
				Value:   zerolog.InfoLevel.String(),
				Usage:   "Log level",
			},
		},
		Commands: []*cli.Command{
			{
				Name:   "version",
				Usage:  "Print version and exit",
				Action: actionVersion,
			},
			{
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
			},
			{
				Name:   "ci",
				Usage:  "Lint CI changes",
				Action: actionCI,
			},
			{
				Name:   "config",
				Usage:  "Parse and print used config",
				Action: actionConfig,
			},
			{
				Name:   "parse",
				Usage:  "Parse a query and print AST, use for debugging or understanding query details",
				Action: actionParse,
			},
		},
	}
}

func main() {
	err := sentry.Init(sentry.ClientOptions{})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to init sentry")
	}
	defer sentry.Flush(time.Second * 10)

	app := newApp()
	err = app.Run(os.Args)
	if err != nil {
		log.Fatal().Err(err).Msg("Execution completed with error(s)s")
		os.Exit(1)
	}
}

func actionVersion(c *cli.Context) (err error) {
	err = initLogger(c.String(logLevelFlag))
	if err != nil {
		return fmt.Errorf("failed to set log level: %w", err)
	}
	fmt.Printf("%s (revision: %s)\n", version, commit)
	return nil
}
