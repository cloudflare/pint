package main

import "github.com/prometheus/client_golang/prometheus"

var (
	checkIterationsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "pint_check_iterations_total",
			Help: "Total number of completed check iterations since pint start",
		},
	)
	checkDuration = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name: "pint_check_duration_seconds",
			Help: "How long did a check took to complete",
		},
		[]string{"check"},
	)
)
