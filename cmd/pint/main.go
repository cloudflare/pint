package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/urfave/cli/v3"

	"github.com/cloudflare/pint/internal/config"
)

const (
	configFlag   = "config"
	logLevelFlag = "log-level"
	enabledFlag  = "enabled"
	disabledFlag = "disabled"
	offlineFlag  = "offline"
	noColorFlag  = "no-color"
	workersFlag  = "workers"
	showDupsFlag = "show-duplicates"
)

var (
	version = "unknown"
	commit  = "unknown"
)

func newApp() *cli.Command {
	return &cli.Command{
		Usage: "Prometheus rule linter/validator.",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    configFlag,
				Aliases: []string{"c"},
				Value:   ".pint.hcl",
				Usage:   "Configuration file to use.",
			},
			&cli.IntFlag{
				Name:    workersFlag,
				Aliases: []string{"w"},
				Value:   10,
				Usage:   "Number of worker threads for running checks.",
			},
			&cli.StringFlag{
				Name:    logLevelFlag,
				Aliases: []string{"l"},
				Value:   slog.LevelInfo.String(),
				Usage:   "Log level.",
			},
			&cli.BoolFlag{
				Name:    noColorFlag,
				Aliases: []string{"n"},
				Value:   false,
				Usage:   "Disable output colouring.",
				Sources: cli.EnvVars("NO_COLOR"),
			},
			&cli.StringSliceFlag{
				Name:    disabledFlag,
				Aliases: []string{"d"},
				Usage:   "List of checks to disable (example: promql/cost).",
			},
			&cli.StringSliceFlag{
				Name:    enabledFlag,
				Aliases: []string{"e"},
				Usage:   "Only enable these checks (example: promql/cost).",
			},
			&cli.BoolFlag{
				Name:    offlineFlag,
				Aliases: []string{"o"},
				Value:   false,
				Usage:   "Disable all check that send live queries to Prometheus servers.",
			},
			&cli.BoolFlag{
				Name:    showDupsFlag,
				Aliases: []string{"s"},
				Value:   false,
				Usage:   "Show all reported problems including the same issue duplicated across multiple rules.",
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

type actionMeta struct {
	cfg       config.Config
	isOffline bool
	workers   int
}

func actionSetup(c *cli.Command) (meta actionMeta, err error) {
	err = initLogger(c.String(logLevelFlag), c.Bool(noColorFlag))
	if err != nil {
		return meta, fmt.Errorf("failed to set log level: %w", err)
	}

	meta.workers = c.Int(workersFlag)
	if meta.workers < 1 {
		return meta, fmt.Errorf("--%s flag must be > 0", workersFlag)
	}

	var fromFile bool
	meta.cfg, fromFile, err = config.Load(c.String(configFlag), c.IsSet(configFlag))
	if err != nil {
		return meta, fmt.Errorf("failed to load config file %q: %w", c.String(configFlag), err)
	}
	if fromFile {
		slog.LogAttrs(context.Background(), slog.LevelDebug, "Adding pint config to the parser exclude list", slog.String("path", c.String(configFlag)))
		meta.cfg.Parser.Exclude = append(meta.cfg.Parser.Exclude, c.String(configFlag))
	}

	meta.cfg.SetDisabledChecks(c.StringSlice(disabledFlag))
	enabled := c.StringSlice(enabledFlag)
	if len(enabled) > 0 {
		meta.cfg.Checks.Enabled = enabled
	}

	if c.Bool(offlineFlag) {
		meta.isOffline = true
		meta.cfg.DisableOnlineChecks()
	}

	return meta, nil
}

func main() {
	ctx := context.Background()
	app := newApp()
	err := app.Run(ctx, os.Args)
	if err != nil {
		slog.LogAttrs(ctx, slog.LevelError, "Execution completed with error(s)", slog.Any("err", err))
		os.Exit(1)
	}
}
