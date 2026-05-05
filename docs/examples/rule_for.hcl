# Example enforcing minimum and maximum 'for' and 'keep_firing_for'
# durations on alerting rules. This helps prevent both flaky alerts
# (too short) and alerts that linger forever (too long).

rule {
  # Only apply to alerting rules.
  match {
    kind = "alerting"
  }

  # Require all alerts to wait at least 5m before firing.
  for {
    comment  = "Alert rules must have for >= 5m to avoid flapping"
    severity = "bug"
    min      = "5m"
  }

  # Do not allow alerts to stay firing for more than 1h after resolving.
  keep_firing_for {
    comment  = "keep_firing_for must not exceed 1h"
    severity = "warning"
    max      = "1h"
  }
}

# Stricter policy for critical alerts.
rule {
  match {
    kind = "alerting"

    label "severity" {
      value = "critical"
    }
  }

  for {
    comment  = "Critical alerts must have for >= 10m"
    severity = "bug"
    min      = "10m"
    max      = "30m"
  }
}
