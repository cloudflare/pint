# Example advanced promql/series configuration.
# This check validates that metrics used in rules exist in Prometheus.
# The options below tune how it handles missing metrics and label values.

prometheus "prod" {
  uri = "https://prometheus-prod.example.com"
}

check "promql/series" {
  # How far back to look when checking if a metric was ever present.
  lookbackRange = "5d"

  # Resolution for look-back queries.
  lookbackStep = "1m"

  # Warn instead of bug for metrics matching these patterns.
  ignoreMetrics = [
    ".*_error",
    ".*_errors",
  ]

  # Some label values only appear dynamically (e.g. HTTP status codes).
  # Tell pint not to report missing values for these labels.
  ignoreLabelsValue = {
    # For any http_requests_total query, ignore missing 'code' values.
    "http_requests_total" = ["code"]

    # Only ignore 'reason' label for errors on the dev environment.
    "http_errors_total{env=\"dev\"}" = ["reason"]
  }

  # If a metric is missing from one Prometheus but present on another,
  # and the matching time series have the 'job="pushgateway"' label,
  # do not report it as missing (pushgateway metrics come and go).
  ignoreMatchingElsewhere = ["{job=\"pushgateway\"}"]

  # Abort checking other Prometheus servers after 5m total.
  fallbackTimeout = "5m"
}
