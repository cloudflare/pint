# Define "prod" Prometheus instance that will only be used for
# rules defined in file matching "alerting/prod/.+" or "recording/prod/.+".
prometheus "prod" {
  uri     = "https://prod.example.com"
  timeout = "30s"
  include = [
    "alerting/prod/.+",
    "recording/prod/.+",
  ]
}

# Extra global configuration for the promql/series check.
check "promql/series" {
  # Don't report missing metrics for any metric with name matching
  # one of the regexp matchers below.
  ignoreMetrics = [
    ".+_error",
    ".+_error_.+",
    ".+_errors",
    ".+_errors_.+",
  ]
}
