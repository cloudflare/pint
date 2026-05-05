# Example alerts/count configuration to catch rules that would fire
# too many alerts. This runs the alert query against Prometheus and
# estimates how many unique alerts it would generate.

prometheus "prod" {
  uri = "https://prometheus-prod.example.com"
}

# Informational count for all alerting rules.
rule {
  match {
    kind = "alerting"
  }

  alerts {
    range   = "1d"
    step    = "1m"
    resolve = "5m"
  }
}

# Fail CI if a new alert rule would immediately fire 50+ alerts.
rule {
  match {
    kind = "alerting"
  }

  alerts {
    range    = "1d"
    step     = "1m"
    resolve  = "5m"
    minCount = 50
    comment  = "This alert would fire 50+ times immediately, narrow the query"
    severity = "bug"
  }
}
