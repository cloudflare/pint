# Example using rule/report to enforce high-level policies.
# Unlike other checks, rule/report always reports a problem for
# every matching rule. Use it to block or warn about whole categories.

# Block all recording rules in the alerts directory.
rule {
  match {
    kind = "recording"
    path = "alerts/.+"
  }

  report {
    comment  = "Recording rules are not allowed in the alerts/ directory"
    severity = "bug"
  }
}

# Warn about any alert that does not have a 'team' label.
rule {
  match {
    kind = "alerting"

    # This match catches rules WITHOUT the team label.
    # We use ignore on the opposite condition to achieve this.
  }

  ignore {
    label "team" {
      value = ".+"
    }
  }

  report {
    comment  = "Every alert must have a 'team' label for routing"
    severity = "warning"
  }
}
