# Example owner validation configuration.
# When running pint with --require-owner, every rule file must have
# an owner comment. The 'owners' block restricts which owner names
# are accepted.

# Only allow owner names that match these regexp patterns.
owners {
  allowed = [
    "sre-.*",
    "platform",
    "observability",
  ]
}

# These are example comments you would add to your Prometheus rule YAML files.
# They are NOT part of the pint HCL config above.
#
# Set owner for the entire file:
#   # pint file/owner sre-oncall
#
# Set owner for a single rule:
#   # pint rule/owner platform
