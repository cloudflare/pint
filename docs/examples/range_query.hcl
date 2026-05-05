# Example enforcing a custom maximum range query duration.
# By default pint already checks against Prometheus retention,
# but this adds an additional policy limit.

prometheus "prod" {
  uri = "https://prometheus-prod.example.com"
}

rule {
  # Limit all range queries to at most 4h, regardless of retention.
  range_query {
    max        = "4h"
    comment    = "Range queries must not exceed 4h to keep query cost low"
    severity   = "bug"
  }
}

# Relaxed limit for recording rules that pre-aggregate metrics.
rule {
  match {
    kind = "recording"
  }

  range_query {
    max        = "1d"
    comment    = "Recording rules may use up to 1d range"
    severity   = "warning"
  }
}
