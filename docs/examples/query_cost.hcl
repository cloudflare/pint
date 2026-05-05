# Example query/cost configuration to enforce query performance budgets.
# This runs the rule expression against Prometheus and reports if the query is too expensive.

prometheus "prod" {
  uri = "https://prometheus-prod.example.com"
}

# Informational cost reporting for all rules.
rule {
  cost {}
}

# Fail CI if a recording rule is too expensive.
rule {
  match {
    kind = "recording"
  }

  cost {
    comment               = "Recording rule query is too expensive"
    severity              = "bug"
    maxPeakSamples        = 300000
    maxTotalSamples       = 1000000
    maxEvaluationDuration = "30s"
  }
}

# Block rules that would create too many output series.
rule {
  cost {
    comment    = "Query returns too many time series, consider aggregating"
    severity   = "bug"
    maxSeries  = 5000
  }
}
