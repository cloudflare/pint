package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"regexp"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/config"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/git"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/promapi"
	"github.com/cloudflare/pint/internal/reporter"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/model"
	"github.com/urfave/cli/v3"
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
	Commands: []*cli.Command{
		{
			Name:  "glob",
			Usage: "Check a list of files or directories (can be a glob).",
			Action: func(_ context.Context, c *cli.Command) error {
				meta, err := actionSetup(c)
				if err != nil {
					return err
				}

				paths := c.Args().Slice()
				if len(paths) == 0 {
					return errors.New("at least one file or directory required")
				}

				slog.Debug("Starting glob watch", slog.Any("paths", paths))
				return actionWatch(c, meta, func(_ context.Context) ([]string, error) {
					return paths, nil
				})
			},
		},
		{
			Name:  "rule_files",
			Usage: "Check the list of rule files from paths loaded by Prometheus.",
			Action: func(_ context.Context, c *cli.Command) error {
				meta, err := actionSetup(c)
				if err != nil {
					return err
				}

				args := c.Args().Slice()
				if len(args) != 1 {
					return errors.New("exactly one argument required with the URI of Prometheus server to query")
				}

				gen := config.NewPrometheusGenerator(meta.cfg, prometheus.NewRegistry())
				if err = gen.GenerateStatic(); err != nil {
					return err
				}

				prom := gen.ServerWithName(args[0])
				if prom == nil {
					return fmt.Errorf("no Prometheus named %q configured in pint", args[0])
				}

				slog.Debug("Starting rule_fules watch", slog.String("name", args[0]))

				return actionWatch(c, meta, func(ctx context.Context) ([]string, error) {
					cfg, err := prom.Config(ctx, time.Millisecond)
					if err != nil {
						return nil, fmt.Errorf("failed to query %q Prometheus configuration: %w", prom.Name(), err)
					}
					return cfg.Config.RuleFiles, nil
				})
			},
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

func actionWatch(c *cli.Command, meta actionMeta, f pathFinderFunc) error {
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
	collector := newProblemCollector(meta.cfg, f, minSeverity, int(c.Int(maxProblemsFlag)), c.Bool(showDupsFlag))
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

	http.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "OK\n")
	})
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

	schema := parseSchema(meta.cfg.Parser.Schema)
	names := parseNames(meta.cfg.Parser.Names)
	allowedOwners := meta.cfg.Owners.CompileAllowed()

	// start timer to run every $interval
	ack := make(chan bool, 1)
	mainCtx, mainCancel := context.WithCancel(context.WithValue(context.Background(), config.CommandKey, config.WatchCommand))
	stop := startTimer(mainCtx, meta.workers, meta.isOffline, gen, schema, names, allowedOwners, interval, ack, collector)

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

func startTimer(ctx context.Context, workers int, isOffline bool, gen *config.PrometheusGenerator, schema parser.Schema, names model.ValidationScheme, allowedOwners []*regexp.Regexp, interval time.Duration, ack chan bool, collector *problemCollector) chan bool {
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
				if err := collector.scan(ctx, workers, isOffline, gen, schema, names, allowedOwners); err != nil {
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
	finder           pathFinderFunc
	fileOwners       map[string]string
	summary          *reporter.Summary
	problem          *prometheus.Desc
	problems         *prometheus.Desc
	fileOwnersMetric *prometheus.Desc
	cfg              config.Config
	minSeverity      checks.Severity
	maxProblems      int
	lock             sync.Mutex
	showDuplicates   bool
}

func newProblemCollector(cfg config.Config, f pathFinderFunc, minSeverity checks.Severity, maxProblems int, showDuplicates bool) *problemCollector {
	return &problemCollector{ // nolint: exhaustruct
		finder:     f,
		cfg:        cfg,
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
		minSeverity:    minSeverity,
		maxProblems:    maxProblems,
		showDuplicates: showDuplicates,
	}
}

func (c *problemCollector) scan(ctx context.Context, workers int, isOffline bool, gen *config.PrometheusGenerator, schema parser.Schema, names model.ValidationScheme, allowedOwners []*regexp.Regexp) error {
	paths, err := c.finder(ctx)
	if err != nil {
		return fmt.Errorf("failed to get the list of paths to check: %w", err)
	}

	slog.Info("Finding all rules to check", slog.Any("paths", paths))
	entries, err := discovery.NewGlobFinder(
		paths,
		git.NewPathFilter(
			config.MustCompileRegexes(c.cfg.Parser.Include...),
			config.MustCompileRegexes(c.cfg.Parser.Exclude...),
			config.MustCompileRegexes(c.cfg.Parser.Relaxed...),
		),
		schema,
		names,
		allowedOwners,
	).Find()
	if err != nil {
		return err
	}

	s, err := checkRules(ctx, workers, isOffline, gen, c.cfg, entries)
	if err != nil {
		return err
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	s.SortReports()
	s.Dedup()

	c.summary = &s

	fileOwners := map[string]string{}
	for _, entry := range entries {
		if entry.Owner != "" {
			fileOwners[entry.Path.SymlinkTarget] = entry.Owner
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

	done := map[string][]prometheus.Metric{}
	keys := []string{}

	for _, report := range c.summary.Reports() {
		if report.Problem.Severity < c.minSeverity {
			slog.Debug(
				"Skipping report with severity lower than minimum configured",
				slog.String("severity", report.Problem.Severity.String()),
				slog.String("minimum", c.minSeverity.String()),
			)
			continue
		}
		if report.IsDuplicate && !c.showDuplicates {
			continue
		}

		var metrics []prometheus.Metric
		for _, diag := range report.Problem.Diagnostics {
			metrics = append(metrics, metricFromProblem(report, c.problem, fmt.Sprintf("%s: %s", report.Problem.Summary, diag.Message)))
		}

		if len(metrics) == 0 {
			metrics = append(metrics, metricFromProblem(report, c.problem, report.Problem.Summary))
		}

		var out dto.Metric
		for _, metric := range metrics {
			// This uses constMetric from client_golang and it never fails.
			_ = metric.Write(&out)
		}

		key := out.String()
		if _, ok := done[key]; !ok {
			done[key] = metrics
			keys = append(keys, key)
		}
	}

	ch <- prometheus.MustNewConstMetric(c.problems, prometheus.GaugeValue, float64(len(done)))

	slices.Sort(keys)
	var reported int
	for _, key := range keys {
		for _, metric := range done[key] {
			ch <- metric
			reported++
			if c.maxProblems > 0 && reported >= c.maxProblems {
				return
			}
		}
	}
}

func metricFromProblem(report reporter.Report, problem *prometheus.Desc, summary string) prometheus.Metric {
	name := report.Rule.Name()
	if name == "" {
		name = "unknown"
	}
	return prometheus.MustNewConstMetric(
		problem,
		prometheus.GaugeValue,
		1,
		report.Path.Name,
		string(report.Rule.Type()),
		name,
		strings.ToLower(report.Problem.Severity.String()),
		report.Problem.Reporter,
		summary,
		report.Owner,
	)
}

type pathFinderFunc func(ctx context.Context) ([]string, error)
