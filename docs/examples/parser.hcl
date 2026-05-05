# Example parser configuration for non-standard rule files.
# Use this when your rules are embedded in other YAML structures
# or when you need Thanos schema support.

parser {
  # Use Thanos rule schema instead of Prometheus.
  # This allows 'partial_response_strategy' on rule groups.
  schema = "thanos"

  # Validate label names using legacy ASCII-only rules.
  # Default is "utf-8".
  names = "legacy"

  # Only check files inside these paths.
  include = [
    "rules/.+",
    "alerts/.+",
  ]

  # Skip markdown and text files that might be in the same directories.
  exclude = [
    ".+\\.md",
    ".+\\.txt",
  ]

  # Parse files matching these patterns in relaxed mode.
  # Relaxed mode finds rule definitions anywhere in the file
  # without requiring strict groups -> rules -> rule structure.
  relaxed = [
    "services/.+\\.yaml",
    "kustomize/.+\\.yml",
  ]
}
