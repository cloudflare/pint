package main

import (
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
