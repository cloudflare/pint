# Example Prometheus configuration with TLS and failover.
# Use this when Prometheus requires client certificates or uses a custom CA.

prometheus "prod" {
  uri      = "https://prometheus-prod.example.com"
  failover = ["https://prometheus-prod-backup.example.com"]

  # Custom headers sent with every API request.
  headers = {
    "X-Auth" = "${ENV_PROMETHEUS_AUTH_TOKEN}"
  }

  # Optional TLS configuration for mTLS connections.
  tls {
    # SNI value for the TLS handshake.
    serverName = "prometheus.example.com"

    # CA certificate to verify the server certificate.
    caCert = "/etc/pint/certs/ca.pem"

    # Client certificate and key for mutual TLS.
    clientCert = "/etc/pint/certs/client.pem"
    clientKey  = "/etc/pint/certs/client.key"
  }

  # Only use this Prometheus for rules under rules/prod/.
  include = ["rules/prod/.+"]
}
