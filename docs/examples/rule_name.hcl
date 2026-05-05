# Example enforcing naming conventions for alerting and recording rules.
# This helps keep rule names consistent and searchable across the organisation.

# Require all recording rules to use a 'rec:' prefix.
rule {
  match {
    kind = "recording"
  }

  name "rec:.+" {
    comment  = "All recording rules must start with 'rec:'"
    severity = "bug"
  }
}

# Require all alerting rules to use snake_case names.
rule {
  match {
    kind = "alerting"
  }

  name "^[a-z_]+$" {
    comment  = "Alert names must use snake_case (lowercase letters and underscores only)"
    severity = "warning"
  }
}
