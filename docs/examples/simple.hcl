# This is a simplified config that only uses a single Prometheus
# server for all checks.

prometheus "prod" {
  uri     = "https://prod.example.com"
  timeout = "1m"
}
