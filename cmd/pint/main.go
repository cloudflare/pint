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
	offlineFlag  = "offline"
	noColorFlag  = "no-color"
	intervalFlag = "interval"
	listenFlag   = "listen"
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
			&cli.BoolFlag{
				Name:    noColorFlag,
				Aliases: []string{"n"},
				Value:   false,
				Usage:   "Disable output colouring",
			},
		},
		Commands: []*cli.Command{
			versionCmd,
			lintCmd,
			ciCmd,
			watchCmd,
			configCmd,
			parseCmd,
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
		log.Fatal().Err(err).Msg("Execution completed with error(s)")
		os.Exit(1)
	}
}
