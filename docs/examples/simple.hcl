# This is a simplified config that only uses "series" check to report any
# alert that is using time series not found in Prometheus server.

prometheus "prod" {
  uri     = "https://prod.example.com"
  timeout = "1m"
}

rule {
  series {
    severity = "bug"
  }
}
