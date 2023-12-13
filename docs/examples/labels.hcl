# This example shows how to enforce labels where the label value itself can
# be a string with one or more sub-strings in it.
# For example when you have a 'components' label that can be any of these:
# components: 'db'
# components: 'api'
# components: 'proxy'
# components: 'db api'
# components: 'db api proxy'
# components: 'proxy api db'
# components: 'proxy db'

rule {
  # Only run these checks on alerting rules.
  # Ignore recording rules.
  match {
    kind = "alerting"
  }
  label "components" {
    # Every alerting rule must have this label set.
    required = true
    # If any alerting rule fails our check pint will report his as a 'Bug'
    # severity problem, which will fail (exit with non-zero exit code)
    # when running 'pint lint' or 'pint ci'.
    # Set it to 'warning' if you don't want to fail pint runs.
    severity = "bug"
    # Split label value into sub-strings using the 'token' regexp.
    # \w is an alias for [0-9A-Za-z_] match.
    # Notice that we must escape '\' in HCL config files.
    token    = "\\w+"
    # This is the list of allowed values.
    values   = [
      "db",
      "api",
      "proxy",
    ]
  }
}
