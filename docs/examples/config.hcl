# Define "prod" Prometheus instance that will only be used for
# rules defined in file matching "alerting/prod/.+" or "recording/prod/.+".
prometheus "prod" {
  uri     = "https://prod.example.com"
  timeout = "30s"

  paths = [
    "alerting/prod/.+",
    "recording/prod/.+",
  ]
}

# Define "dev" Prometheus instance that will be use for all rule checks.
prometheus "dev" {
  uri     = "https://dev.example.com"
  timeout = "60s"
}

rule {
  # Disallow spaces in label/annotation keys, they're only allowed in values.
  reject ".* +.*" {
    label_keys      = true
    annotation_keys = true
  }

  # Disallow URLs in labels, they should go to annotations.
  reject "https?://.+" {
    label_keys   = true
    label_values = true
  }
}

rule {
  # This block will apply to all alerting rules.
  match {
    kind = "alerting"
  }

  # Each alert must have a 'summary' annotation on every alert.
  annotation "summary" {
    severity = "bug"
    required = true
  }

  # Each alert must have a 'dashboard' annotation that links to grafana.
  annotation "dashboard" {
    severity = "bug"
    value    = "https://grafana.example.com/(.+)"
  }

  # Each alert must have a 'severity' annotation that's either 'critical' or 'warning'.
  label "severity" {
    severity = "bug"
    value    = "(critical|warning)"
    required = true
  }

  # Check how many times each alert would fire in the last 1d.
  alerts {
    range   = "1d"
    step    = "1m"
    resolve = "5m"
  }

  # Check if '{{ $value }}'/'{{ .Value }}' is used in labels
  # https://www.robustperception.io/dont-put-the-value-in-alert-labels
  value {}

  # Check if templates used in annotations and labels are valid.
  template {}
}

rule {
  # This block will apply to all alerting rules with severity="critical" label set.
  match {
    kind = "alerting"

    label "severity" {
      value = "critical"
    }
  }

  # All severity="critical" alerts must have a runbook link as annotation.
  annotation "runbook" {
    severity = "bug"
    value    = "https://runbook.example.com/.+"
    required = true
  }
}

rule {
  # This block will apply to all recording rules.
  match {
    kind = "recording"
  }

  # Ensure that all aggregations are preserving "job" label.
  aggregate ".+" {
    severity = "bug"
    keep     = ["job"]
  }

  # Enable series check that will warn if alert rules are using time series
  # not present in any of configured Prometheus servers.
  series {}

  # Enable cost checks that will print the number of returned time series and try
  # to estimate total memory usage assuming that each series require 4KB of memory.
  cost {
    bytesPerSample = 4096
  }
}

rule {
  # This block will apply to all recording rules in "recording/federation" directory.
  match {
    kind = "recording"
    path = "recording/federation/.+"
  }

  # All recording rules named "cluster:.+" must strip "instance" label when aggregating.
  # Example rule that would raise a linter error:
  # - record: cluster:http_requests:rate5m
  #   expr: sum(rate(http_requests_total[5m])) by (job, instance)
  # Rules that would be allowed:
  # - record: cluster:http_requests:rate5m
  #   expr: sum(rate(http_requests_total[5m])) by (job)
  # - record: cluster:http_requests:rate5m
  #   expr: sum(rate(http_requests_total[5m]))
  aggregate "cluster:.+" {
    severity = "bug"
    strip    = ["instance"]
  }
}
