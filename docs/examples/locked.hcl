# Example of locked rules.
# When a rule is locked, users cannot disable it using
# # pint disable ... or # pint snooze ... comments.
# This is useful for policy checks that must always run.

# Require all alerts to have a 'severity' label.
# Because the rule is locked, engineers cannot bypass it with comments.
rule {
  locked = true

  match {
    kind = "alerting"
  }

  label "severity" {
    required = true
    value    = "(critical|warning|info)"
    severity = "bug"
  }
}
