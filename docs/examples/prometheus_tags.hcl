# Example using tags on Prometheus servers.
# Tags let you group servers and disable checks for entire groups
# using # pint disable ...(+tag) comments.

# Two production Prometheus servers sharing the same tag.
prometheus "prod-us" {
  uri  = "https://prometheus-us.example.com"
  tags = ["prod"]
}

prometheus "prod-eu" {
  uri  = "https://prometheus-eu.example.com"
  tags = ["prod"]
}

# A staging server with its own tag.
prometheus "staging" {
  uri  = "https://prometheus-staging.example.com"
  tags = ["staging"]
}

# Disable expensive promql/series checks on all staging servers
# by using the +staging tag in a comment.
#
# In your rule YAML file you would add:
#   # pint disable promql/series(+staging)
#
# Or disable for all production servers:
#   # pint disable promql/series(+prod)
