# Example using templated regular expressions.
# Some checks allow regexp patterns to reference rule fields using
# Go text/template syntax. Exposed fields are:
#   $alert, $record, $for, $labels.<name>, $annotations.<name>

# Reject label or annotation values that are identical to the alert name.
# This prevents redundant labels like "alertname: foo" on an alert named "foo".
rule {
  match {
    kind = "alerting"
  }

  reject "{{ $alert }}" {
    comment           = "Do not repeat the alert name in labels or annotations"
    label_values      = true
    annotation_values = true
    severity          = "warning"
  }
}

# Require all recording rule names to contain the value of the "team" label.
# If a rule has labels: { team: "platform" }, the name must include "platform".
rule {
  match {
    kind = "recording"

    label "team" {
      value = ".+"
    }
  }

  name "{{ $labels.team }}:.+" {
    comment  = "Recording rule name must start with the team label value"
    severity = "bug"
  }
}

# Require that every alert has a runbook_url annotation matching:
# docs.example.com/alerts/<alert>.html
rule {
  match {
    kind = "alerting"
  }

  annotation "runbook_url" {
    required = true
    comment  = "Runbook URL must be docs.example.com/alerts/<alert>.html"
    severity = "bug"
    value    = "https://docs\\.example\\.com/alerts/{{ $alert }}\\.html"
  }
}
