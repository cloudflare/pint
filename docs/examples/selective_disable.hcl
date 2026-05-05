# Example: selectively enable or disable checks for specific rules.
# This is the config-file alternative to adding # pint disable ... comments.
# It is useful when:
#   - Rules are auto-generated and you cannot add comments to them.
#   - The same exception applies to many rules.
#   - You want all linting policy exceptions in one place.
# See: https://github.com/cloudflare/pint/discussions/830

# --- Disabling checks with advanced match features ---

# Disable promql/series only for rules that are being ADDED in a PR.
# Existing (unmodified) rules still get the check, so broken edits are caught.
rule {
  match {
    state = ["added"]
    command = "ci"
  }

  disable = ["promql/series"]
}

# Disable rule/for for alerts with long 'for' durations.
# These are typically slow-burn alerts where 'for' is intentionally long.
rule {
  match {
    kind = "alerting"
    for  = "> 30m"
  }

  disable = ["rule/for"]
}

# Disable alerts/count for alerts that already have a validated runbook.
# We trust alerts with a runbook URL to be well-scoped.
rule {
  match {
    kind = "alerting"

    annotation "runbook_url" {
      value = ".+"
    }
  }

  disable = ["alerts/count"]
}

# Disable promql/rate for the platform team's rules only.
# Other teams still get the full check.
rule {
  match {
    label "team" {
      value = "platform"
    }
  }

  disable = ["promql/rate"]
}

# Disable rule/link for rules in the staging directory,
# but NOT for rules that also have the "critical" label.
rule {
  match {
    path = "rules/staging/.+"
  }

  ignore {
    label "severity" {
      value = "critical"
    }
  }

  disable = ["rule/link"]
}

# --- Enabling checks for specific rules ---

# Disable promql/rate globally, then re-enable it only for critical alerts.
checks {
  disabled = ["promql/rate"]
}

rule {
  match {
    path = "rules/critical/.+"
    kind = "alerting"
  }

  enable = ["promql/rate"]
}
