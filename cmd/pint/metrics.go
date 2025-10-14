package main

import "github.com/prometheus/client_golang/prometheus"

var (
	metricsRegistry = prometheus.NewRegistry()

	pintVersion = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pint_version",
			Help: "Version information.",
		},
		[]string{"version"},
	)
	checkIterationsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "pint_check_iterations_total",
			Help: "Total number of completed check iterations since pint start.",
		},
	)
	checkIterationChecks = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "pint_last_run_checks",
			Help: "The number of checks to run in the current iteration.",
		},
	)
	checkIterationChecksDone = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "pint_last_run_checks_done",
			Help: "The number of checks completed in the current iteration.",
		},
	)
	checkDuration = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name: "pint_check_duration_seconds",
			Help: "How long did a check took to complete.",
		},
		[]string{"check"},
	)
	lastRunTime = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "pint_last_run_time_seconds",
			Help: "Last checks run completion time since unix epoch in seconds.",
		},
	)
	lastRunDuration = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "pint_last_run_duration_seconds",
			Help: "Last checks run duration in seconds.",
		},
	)
	rulesParsedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pint_rules_parsed_total",
			Help: "Total number of rules parsed since startup.",
		},
		[]string{"kind"},
	)
)
