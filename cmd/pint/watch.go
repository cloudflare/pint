package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/config"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/git"
	"github.com/cloudflare/pint/internal/promapi"
	"github.com/cloudflare/pint/internal/reporter"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	dto "github.com/prometheus/client_model/go"
	"github.com/urfave/cli/v2"
)

const (
	intervalFlag    = "interval"
	listenFlag      = "listen"
	pidfileFlag     = "pidfile"
	maxProblemsFlag = "max-problems"
	minSeverityFlag = "min-severity"
)

var watchCmd = &cli.Command{
	Name:  "watch",
	Usage: "Run in the foreground and continuesly check specified rules.",
	Subcommands: []*cli.Command{
		{
			Name:   "glob",
			Usage:  "Check a list of files or directories (can be a glob).",
			Action: actionWatch,
		},
	},
	Flags: []cli.Flag{
		&cli.DurationFlag{
			Name:    intervalFlag,
			Aliases: []string{"i"},
			Value:   time.Minute * 10,
			Usage:   "How often to run all checks.",
		},
		&cli.StringFlag{
			Name:    listenFlag,
			Aliases: []string{"s"},
			Value:   ":8080",
			Usage:   "Listen address for HTTP web server exposing metrics.",
		},
		&cli.StringFlag{
			Name:    pidfileFlag,
			Aliases: []string{"p"},
			Usage:   "Write pid file to this path.",
		},
		&cli.IntFlag{
			Name:    maxProblemsFlag,
			Aliases: []string{"m"},
			Value:   0,
			Usage:   "Maximum number of problems to report on metrics, 0 - no limit.",
		},
		&cli.StringFlag{
			Name:    minSeverityFlag,
			Aliases: []string{"n"},
			Value:   strings.ToLower(checks.Bug.String()),
			Usage:   "Set minimum severity for problems reported via metrics.",
		},
	},
}

func actionWatch(c *cli.Context) error {
	meta, err := actionSetup(c)
	if err != nil {
		return err
	}

	paths := c.Args().Slice()
	if len(paths) == 0 {
		return fmt.Errorf("at least one file or directory required")
	}

	minSeverity, err := checks.ParseSeverity(c.String(minSeverityFlag))
	if err != nil {
		return fmt.Errorf("invalid --%s value: %w", minSeverityFlag, err)
	}

	pidfile := c.String(pidfileFlag)
	if pidfile != "" {
		pid := os.Getpid()
		err = os.WriteFile(pidfile, []byte(fmt.Sprintf("%d\n", pid)), 0o644)
		if err != nil {
			return err
		}
		slog.Info("Pidfile created", slog.String("path", pidfile))
		defer func() {
			pidErr := os.RemoveAll(pidfile)
			if pidErr != nil {
				slog.Error("Failed to remove pidfile", slog.Any("err", pidErr), slog.String("path", pidfile))
			}
			slog.Info("Pidfile removed", slog.String("path", pidfile))
		}()
	}

	// start HTTP server for metrics
	collector := newProblemCollector(meta.cfg, paths, minSeverity, c.Int(maxProblemsFlag))
	// register all metrics
	metricsRegistry.MustRegister(collector)
	metricsRegistry.MustRegister(checkDuration)
	metricsRegistry.MustRegister(checkIterationsTotal)
	metricsRegistry.MustRegister(checkIterationChecks)
	metricsRegistry.MustRegister(checkIterationChecksDone)
	metricsRegistry.MustRegister(pintVersion)
	metricsRegistry.MustRegister(lastRunTime)
	metricsRegistry.MustRegister(lastRunDuration)
	metricsRegistry.MustRegister(rulesParsedTotal)
	promapi.RegisterMetrics(metricsRegistry)

	metricsRegistry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	// init metrics if needed
	pintVersion.WithLabelValues(version).Set(1)
	rulesParsedTotal.WithLabelValues(config.AlertingRuleType).Add(0)
	rulesParsedTotal.WithLabelValues(config.RecordingRuleType).Add(0)
	rulesParsedTotal.WithLabelValues(config.InvalidRuleType).Add(0)

	http.Handle("/metrics", promhttp.HandlerFor(metricsRegistry, promhttp.HandlerOpts{
		ErrorLog: slog.NewLogLogger(slog.Default().Handler(), slog.LevelError),
		Timeout:  time.Second * 20,
	}))
	listen := c.String(listenFlag)
	server := http.Server{
		Addr:         listen,
		ReadTimeout:  time.Second * 30,
		WriteTimeout: time.Second * 30,
	}
	go func() {
		if httpErr := server.ListenAndServe(); !errors.Is(httpErr, http.ErrServerClosed) {
			slog.Error("HTTP server returned an error", slog.Any("err", httpErr), slog.String("listen", listen))
		}
	}()
	slog.Info("Started HTTP server", slog.String("address", listen))

	interval := c.Duration(intervalFlag)

	gen := config.NewPrometheusGenerator(meta.cfg, metricsRegistry)
	if err = gen.GenerateStatic(); err != nil {
		return err
	}

	// start timer to run every $interval
	ack := make(chan bool, 1)
	mainCtx, mainCancel := context.WithCancel(context.WithValue(context.Background(), config.CommandKey, config.WatchCommand))
	stop := startTimer(mainCtx, meta.workers, meta.isOffline, gen, interval, ack, collector)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down")
	mainCancel()

	stop <- true
	slog.Info("Waiting for all background tasks to finish")
	<-ack

	gen.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err = server.Shutdown(ctx); err != nil {
		slog.Error("HTTP server returned an error while shutting down", slog.Any("err", err))
	}

	return nil
}

func startTimer(ctx context.Context, workers int, isOffline bool, gen *config.PrometheusGenerator, interval time.Duration, ack chan bool, collector *problemCollector) chan bool {
	ticker := time.NewTicker(time.Second)
	stop := make(chan bool, 1)
	wasBootstrapped := false

	go func() {
		for {
			select {
			case <-ticker.C:
				slog.Debug("Running checks")
				if !wasBootstrapped {
					ticker.Reset(interval)
					wasBootstrapped = true
				}
				if err := collector.scan(ctx, workers, isOffline, gen); err != nil {
					slog.Error("Got an error when running checks", slog.Any("err", err))
				}
				checkIterationsTotal.Inc()
			case <-stop:
				ticker.Stop()
				slog.Info("Background worker finished")
				ack <- true
				return
			}
		}
	}()
	slog.Info("Will continuously run checks until terminated", slog.String("interval", interval.String()))

	return stop
}

type problemCollector struct {
	cfg              config.Config
	fileOwners       map[string]string
	summary          *reporter.Summary
	problem          *prometheus.Desc
	problems         *prometheus.Desc
	fileOwnersMetric *prometheus.Desc
	paths            []string
	minSeverity      checks.Severity
	maxProblems      int
	lock             sync.Mutex
}

func newProblemCollector(cfg config.Config, paths []string, minSeverity checks.Severity, maxProblems int) *problemCollector {
	return &problemCollector{
		cfg:        cfg,
		paths:      paths,
		fileOwners: map[string]string{},
		problem: prometheus.NewDesc(
			"pint_problem",
			"Prometheus rule problem reported by pint",
			[]string{"filename", "kind", "name", "severity", "reporter", "problem", "owner"},
			prometheus.Labels{},
		),
		problems: prometheus.NewDesc(
			"pint_problems",
			"Total number of problems reported by pint",
			[]string{},
			prometheus.Labels{},
		),
		fileOwnersMetric: prometheus.NewDesc(
			"pint_rule_file_owner",
			"This is a boolean metric that describes who is the configured owner for given rule file",
			[]string{"filename", "owner"},
			prometheus.Labels{},
		),
		minSeverity: minSeverity,
		maxProblems: maxProblems,
	}
}

func (c *problemCollector) scan(ctx context.Context, workers int, isOffline bool, gen *config.PrometheusGenerator) error {
	slog.Info("Finding all rules to check", slog.Any("paths", c.paths))
	entries, err := discovery.NewGlobFinder(c.paths, git.NewPathFilter(nil, nil, c.cfg.Parser.CompileRelaxed())).Find()
	if err != nil {
		return err
	}

	s, err := checkRules(ctx, workers, isOffline, gen, c.cfg, entries)
	if err != nil {
		return err
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	c.summary = &s

	fileOwners := map[string]string{}
	for _, entry := range entries {
		if entry.Owner != "" {
			fileOwners[entry.ReportedPath] = entry.Owner
		}
	}
	c.fileOwners = fileOwners

	return nil
}

func (c *problemCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.problem
}

func (c *problemCollector) Collect(ch chan<- prometheus.Metric) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.summary == nil {
		return
	}

	for filename, owner := range c.fileOwners {
		ch <- prometheus.MustNewConstMetric(c.fileOwnersMetric, prometheus.GaugeValue, 1, filename, owner)
	}

	done := map[string]prometheus.Metric{}
	keys := []string{}

	for _, report := range c.summary.Reports() {
		if report.Problem.Severity < c.minSeverity {
			slog.Debug("Skipping report", slog.String("severity", report.Problem.Severity.String()))
			continue
		}

		kind := "invalid"
		name := "unknown"
		if report.Rule.AlertingRule != nil {
			kind = "alerting"
			name = report.Rule.AlertingRule.Alert.Value
		}
		if report.Rule.RecordingRule != nil {
			kind = "recording"
			name = report.Rule.RecordingRule.Record.Value
		}
		metric := prometheus.MustNewConstMetric(
			c.problem,
			prometheus.GaugeValue,
			1,
			report.SourcePath,
			kind,
			name,
			strings.ToLower(report.Problem.Severity.String()),
			report.Problem.Reporter,
			report.Problem.Text,
			report.Owner,
		)

		var out dto.Metric
		err := metric.Write(&out)
		if err != nil {
			slog.Error("Failed to write metric to a buffer", slog.Any("err", err))
			continue
		}

		key := out.String()
		if _, ok := done[key]; !ok {
			done[key] = metric
			keys = append(keys, key)
		}
	}

	ch <- prometheus.MustNewConstMetric(c.problems, prometheus.GaugeValue, float64(len(done)))

	sort.Strings(keys)
	var reported int
	for _, key := range keys {
		ch <- done[key]
		reported++
		if c.maxProblems > 0 && reported >= c.maxProblems {
			break
		}
	}
}
