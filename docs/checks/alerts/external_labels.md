---
layout: default
parent: Checks
grand_parent: Documentation
---

# alerts/external_labels

Alerting rules can be templated to render the value of external labels
configured for the Prometheus server these rules are being evaluated
using `$externalLabels` variable.
[See docs](https://prometheus.io/docs/prometheus/latest/configuration/template_reference/#alert-field-templates).

This check will look for alerting rules referencing external labels that are
not present on given Prometheus server.

If we define `cluster` label in `global:external_labels`, example:

```yaml
global:
  external_labels:
    cluster: mycluster
```

Then we can access it in alert rules deployed to that Prometheus server
by using `$externalLabels.cluster` variable:

```yaml
- alert: Abc Is Down
  expr: up{job="abc"} == 0
  annotations:
    summary: "{{ $labels.job }} is down in {{ $externalLabels.cluster }} cluster"
```

But if we try to do that without `cluster` in `global:external_labels` configuration
then `$externalLabels.cluster` will be empty, and this is what this check would
report.

## Configuration

This check doesn't have any configuration options.

## How to enable it

This check is enabled by default for all configured Prometheus servers.

Example:

```js
prometheus "prod" {
  uri     = "https://prometheus-prod.example.com"
  timeout = "60s"
  include = [
    "rules/prod/.*",
    "rules/common/.*",
  ]
}

prometheus "dev" {
  uri     = "https://prometheus-dev.example.com"
  timeout = "30s"
  include = [
    "rules/dev/.*",
    "rules/common/.*",
  ]
}
```

## How to disable it

You can disable this check globally by adding this config block:

```js
checks {
  disabled = ["alerts/external_labels"]
}
```

You can also disable it for all rules inside given file by adding
a comment anywhere in that file. Example:

```yaml
# pint file/disable alerts/external_labels
```

Or you can disable it per rule by adding a comment to it. Example:

```yaml
# pint disable alerts/external_labels
```

If you want to disable only individual instances of this check
you can add a more specific comment.

```yaml
# pint disable alerts/external_labels($prometheus)
```

Where `$prometheus` is the name of Prometheus server to disable.

Example:

```yaml
# pint disable alerts/external_labels(prod)
```

## How to snooze it

You can disable this check until given time by adding a comment to it. Example:

```yaml
# pint snooze $TIMESTAMP alerts/external_labels
```

Where `$TIMESTAMP` is either use [RFC3339](https://www.rfc-editor.org/rfc/rfc3339)
formatted or `YYYY-MM-DD`.
Adding this comment will disable `alerts/external_labels` _until_ `$TIMESTAMP`, after that
check will be re-enabled.
