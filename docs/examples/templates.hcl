# Examples of rules using templates.

# Any alert using non-zero `for` field must also
# have a label named `alert_for` with the value
# equal to the `for` value.
# Alert defined as follows:
# - alert: service_down
#   expr: up == 0
#   for: 5m
# would fail this check, but this version wouldn't:
# - alert: service_down
#   expr: up == 0
#   for: 5m
#   labels:
#     alert_for: 5m
rule {
  match {
    for = "> 0"
  }

  label "alert_for" {
    required = true
    value    = "{{ $for }}"
  }
}
