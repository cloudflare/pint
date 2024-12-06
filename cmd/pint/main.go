package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/urfave/cli/v2"
	"go.uber.org/automaxprocs/maxprocs"

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
)

var (
	version = "unknown"
	commit  = "unknown"
)

func newApp() *cli.App {
	return &cli.App{
		Usage: "Prometheus rule linter/validator.",
		Flags: []cli.Flag{
			&cli.PathFlag{
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
				EnvVars: []string{"NO_COLOR"},
			},
			&cli.StringSliceFlag{
				Name:    disabledFlag,
				Aliases: []string{"d"},
				Value:   cli.NewStringSlice(),
				Usage:   "List of checks to disable (example: promql/cost).",
			},
			&cli.StringSliceFlag{
				Name:    enabledFlag,
				Aliases: []string{"e"},
				Value:   cli.NewStringSlice(),
				Usage:   "Only enable these checks (example: promql/cost).",
			},
			&cli.BoolFlag{
				Name:    offlineFlag,
				Aliases: []string{"o"},
				Value:   false,
				Usage:   "Disable all check that send live queries to Prometheus servers.",
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

func actionSetup(c *cli.Context) (meta actionMeta, err error) {
	err = initLogger(c.String(logLevelFlag), c.Bool(noColorFlag))
	if err != nil {
		return meta, fmt.Errorf("failed to set log level: %w", err)
	}

	undo, err := maxprocs.Set()
	defer undo()
	if err != nil {
		slog.Error("failed to set GOMAXPROCS", slog.Any("err", err))
	}

	meta.workers = c.Int(workersFlag)
	if meta.workers < 1 {
		return meta, fmt.Errorf("--%s flag must be > 0", workersFlag)
	}

	var fromFile bool
	meta.cfg, fromFile, err = config.Load(c.Path(configFlag), c.IsSet(configFlag))
	if err != nil {
		return meta, fmt.Errorf("failed to load config file %q: %w", c.Path(configFlag), err)
	}
	if fromFile {
		slog.Debug("Adding pint config to the parser exclude list", slog.String("path", c.Path(configFlag)))
		meta.cfg.Parser.Exclude = append(meta.cfg.Parser.Exclude, c.Path(configFlag))
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
	app := newApp()
	err := app.Run(os.Args)
	if err != nil {
		slog.Error("Execution completed with error(s)", slog.Any("err", err))
		os.Exit(1)
	}
}
