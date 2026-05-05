# Example of Prometheus server discovery using a PromQL query.
# This is useful when Prometheus instances are registered in a service
# discovery system and their addresses are available as metrics.

discovery {
  # Query an existing Prometheus server to discover other Prometheus instances.
  prometheusQuery {
    uri     = "https://prometheus-discovery.example.com"
    timeout = "2m"

    # The PromQL query must return time series with labels we can use
    # in the template below. Here we assume each Prometheus instance
    # exposes a 'prometheus_build_info' metric with an 'instance' label.
    query = "prometheus_build_info"

    template {
      # Use the 'instance' label from query results as the server name.
      name = "{{ $instance }}"

      # Build the URI from the discovered instance label.
      uri = "https://{{ $instance }}"

      # Tag all discovered servers so they can be referenced together.
      tags = ["discovered"]

      # Only check rules that belong to this discovered Prometheus.
      include = ["rules/{{ $instance }}/.+"]
    }
  }
}
