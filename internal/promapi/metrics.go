package promapi

import (
	"errors"
	"fmt"
	"net"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	prometheusQueriesRunning = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pint_prometheus_queries_running",
			Help: "Total number of in-flight prometheus queries.",
		},
		[]string{"name", "endpoint"},
	)
	prometheusQueriesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pint_prometheus_queries_total",
			Help: "Total number of all prometheus queries.",
		},
		[]string{"name", "endpoint"},
	)
	prometheusQueryErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pint_prometheus_query_errors_total",
			Help: "Total number of failed prometheus queries.",
		},
		[]string{"name", "endpoint", "reason"},
	)
)

func RegisterMetrics(reg *prometheus.Registry) {
	reg.MustRegister(prometheusQueriesRunning)
	reg.MustRegister(prometheusQueriesTotal)
	reg.MustRegister(prometheusQueryErrorsTotal)
}

func errReason(err error) string {
	var neterr net.Error
	if ok := errors.As(err, &neterr); ok && neterr.Timeout() {
		return "connection/timeout"
	}

	var e1 APIError
	if ok := errors.As(err, &e1); ok {
		return fmt.Sprintf("api/%s", e1.ErrorType)
	}

	return "connection/error"
}
