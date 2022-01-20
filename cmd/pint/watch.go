package main

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/cloudflare/pint/internal/config"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/output"
	"github.com/cloudflare/pint/internal/reporter"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	dto "github.com/prometheus/client_model/go"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"
)

const (
	intervalFlag = "interval"
	listenFlag   = "listen"
	pidfileFlag  = "pidfile"
)

var watchCmd = &cli.Command{
	Name:   "watch",
	Usage:  "Continuously lint specified files",
	Action: actionWatch,
	Flags: []cli.Flag{
		&cli.DurationFlag{
			Name:    intervalFlag,
			Aliases: []string{"i"},
			Value:   time.Minute * 10,
			Usage:   "How often to run all checks",
		},
		&cli.StringFlag{
			Name:    listenFlag,
			Aliases: []string{"s"},
			Value:   ":8080",
			Usage:   "Listen address for HTTP web server exposing metrics",
		},
		&cli.StringFlag{
			Name:    pidfileFlag,
			Aliases: []string{"p"},
			Usage:   "Write pid file to this path",
		},
	},
}

func actionWatch(c *cli.Context) (err error) {
	err = initLogger(c.String(logLevelFlag), c.Bool(noColorFlag))
	if err != nil {
		return fmt.Errorf("failed to set log level: %w", err)
	}

	paths := c.Args().Slice()
	if len(paths) == 0 {
		return fmt.Errorf("at least one file or directory required")
	}

	cfg, err := config.Load(c.Path(configFlag), c.IsSet(configFlag))
	if err != nil {
		return fmt.Errorf("failed to load config file %q: %w", c.Path(configFlag), err)
	}
	cfg.SetDisabledChecks(c.StringSlice(disabledFlag))
	if c.Bool(offlineFlag) {
		cfg.DisableOnlineChecks()
	}

	pidfile := c.String(pidfileFlag)
	if pidfile != "" {
		pid := os.Getpid()
		err := ioutil.WriteFile(pidfile, []byte(fmt.Sprintf("%d\n", pid)), 0o644)
		if err != nil {
			return err
		}
		log.Info().Str("path", pidfile).Msg("Pidfile created")
		defer func() {
			err := os.RemoveAll(pidfile)
			if err != nil {
				log.Error().Err(err).Str("path", pidfile).Msg("Failed to remove pidfile")
			}
			log.Info().Str("path", pidfile).Msg("Pidfile removed")
		}()
	}

	// start HTTP server for metrics
	collector := newProblemCollector(cfg, paths)
	prometheus.MustRegister(collector)
	prometheus.MustRegister(checkDuration)
	prometheus.MustRegister(checkIterationsTotal)
	http.Handle("/metrics", promhttp.Handler())
	listen := c.String(listenFlag)
	server := http.Server{
		Addr:         listen,
		ReadTimeout:  time.Second * 30,
		WriteTimeout: time.Second * 30,
	}
	go func() {
		if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Error().Err(err).Str("listen", listen).Msg("HTTP server returned an error")
		}
	}()
	log.Info().Str("address", listen).Msg("Started HTTP server")

	mainCtx, cancel := context.WithCancel(context.WithValue(context.Background(), config.CommandKey, config.WatchCommand))

	// start timer to run every $interval
	interval := c.Duration(intervalFlag)
	ack := make(chan bool, 1)
	stop := startTimer(mainCtx, cfg, interval, ack, collector)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("Shutting down")
	cancel()

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err = server.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("HTTP server returned an error while shutting down")
	}

	stop <- true
	log.Info().Msg("Waiting for all background tasks to finish")
	<-ack

	return nil
}

func startTimer(ctx context.Context, cfg config.Config, interval time.Duration, ack chan bool, collector *problemCollector) chan bool {
	ticker := time.NewTicker(time.Second)
	stop := make(chan bool, 1)
	wasBootstrapped := false

	go func() {
		for {
			select {
			case <-ticker.C:
				log.Debug().Msg("Clearing cache")
				cfg.ClearCache()
				log.Debug().Msg("Running checks")
				if !wasBootstrapped {
					ticker.Reset(interval)
					wasBootstrapped = true
				}
				if err := collector.scan(ctx); err != nil {
					log.Error().Err(err).Msg("Got an error when running checks")
				}
				checkIterationsTotal.Inc()
			case <-stop:
				ticker.Stop()
				log.Info().Msg("Background worker finished")
				ack <- true
				return
			}
		}
	}()
	log.Info().Stringer("interval", interval).Msg("Will continuously run checks until terminated")

	return stop
}

type problemCollector struct {
	lock    sync.Mutex
	cfg     config.Config
	paths   []string
	summary *reporter.Summary
	problem *prometheus.Desc
}

func newProblemCollector(cfg config.Config, paths []string) *problemCollector {
	return &problemCollector{
		cfg:   cfg,
		paths: paths,
		problem: prometheus.NewDesc(
			"pint_problem",
			"Prometheus rule problem reported by pint",
			[]string{"kind", "name", "severity", "reporter", "problem", "lines"},
			prometheus.Labels{},
		),
	}
}

func (c *problemCollector) scan(ctx context.Context) error {
	d := discovery.NewGlobFileFinder()
	toScan, err := d.Find(c.paths...)
	if err != nil {
		return err
	}

	if len(toScan.Paths()) == 0 {
		return fmt.Errorf("no matching files")
	}

	s := scanFiles(ctx, c.cfg, toScan, &discovery.NoopLineFinder{})

	c.lock.Lock()
	c.summary = &s
	c.lock.Unlock()

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

	done := map[string]struct{}{}

	for _, report := range c.summary.Reports {
		kind := "invalid"
		name := "unknown"
		if report.Rule.AlertingRule != nil {
			kind = "alerting"
			name = report.Rule.AlertingRule.Alert.Value.Value
		}
		if report.Rule.RecordingRule != nil {
			kind = "recording"
			name = report.Rule.RecordingRule.Record.Value.Value
		}
		metric := prometheus.MustNewConstMetric(
			c.problem,
			prometheus.GaugeValue,
			1,
			kind,
			name,
			strings.ToLower(report.Problem.Severity.String()),
			report.Problem.Reporter,
			report.Problem.Text,
			output.FormatLineRangeString(report.Problem.Lines),
		)

		var out dto.Metric
		err := metric.Write(&out)
		if err != nil {
			log.Error().Err(err).Msg("Failed to write metric to a buffer")
			continue
		}

		key := out.String()
		if _, ok := done[key]; ok {
			continue
		}

		ch <- metric
		done[key] = struct{}{}
	}
}
