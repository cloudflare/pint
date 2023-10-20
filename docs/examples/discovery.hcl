# Example with Prometheus server discovery.

discovery {

  # filepath discovery will generate Prometheus servers from files on disk.
  # We define a regexp and any file or directory path matching that regexp will
  # generate a new Prometheus server.
  filepath {
    # Directory to scan for files.
    directory = "/etc/prometheus/servers"

    # Regexp rule to match, with capture groups to store variables.
    match = "(?P<name>\\w+).yaml"
    
    # Use variables from the regex to generate a new Prometheus configuration block.
    template {
      name     = "prometheus-{{ $name }}" # We can use 'name' regexp capture group as $name.
      uri      = "https://prometheus-{{ $name }}.example.com"
      failover = [ "https://prometheus-{{ $name }}-backup.example.com" ]
      headers  = {
        "X-Auth": "secret",
        "X-User": "bob"
        "X-Cluster": "{{ $name }}"
      }
      timeout = "30s"
    }

    template {
      name     = "prometheus-clone-{{ $name }}"
      uri      = "https://{{ $name }}.example.com"
      failover = [ "https://{{ $name }}-backup.example.com" ]
      headers  = {
        "X-Auth": "secret",
        "X-User": "bob",
      }
      timeout = "30s"
    }
  }
}
