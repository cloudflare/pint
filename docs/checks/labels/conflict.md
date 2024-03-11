---
layout: default
parent: Checks
grand_parent: Documentation
---

# labels/conflict

This check will look for any conflicting labels used in rules.
Below is the list of conflicts it looks for.

## External labels

### Recording rules

If any recording rules are manually setting some labels that are
already present in `external_labels` Prometheus configuration option
then both labels might conflict when metrics are ingested to another
Prometheus via federation or remote read/write.

Example:

Consider this recording rule:

```yaml
groups:
  name: recording rules
  rules:
  - record: prometheus_http_requests_total:rate2m
    expr: rate(prometheus_http_requests_total[2m])
    labels:
      cluster: dev
```

If this rule is deployed to Prometheus server with this configuration:

```yaml
global:
  external_labels:
    site: site01
    cluster: staging
```

Then making a `/federate` request will return time series with `cluster="staging"` label,
except for `prometheus_http_requests_total:rate2m` time series which will have `cluster="dev"`
label from the recording rule, which might cause unexpected inconsistencies.

If both the recording rule and `external_labels` config section uses same label value for the
`cluster` label then this effectively makes `cluster` label redundant, so in both cases it's
best to avoid setting labels used in `external_labels` on individual rules.

### Alerting rules

Same problem exists for alerting rules, with the only difference being how the label is
being used.
Any label listed in `external_labels` will be added to all firing alerts.
Setting `cluster` label on alerting rule will override `external_labels` and
can cause confusion when alert sent from `cluster="staging"` Prometheus has `cluster="dev"`
label set.

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
  disabled = ["labels/conflict"]
}
```

You can also disable it for all rules inside given file by adding
a comment anywhere in that file. Example:

```yaml
# pint file/disable labels/conflict
```

Or you can disable it per rule by adding a comment to it. Example:

```yaml
# pint disable labels/conflict
```

If you want to disable only individual instances of this check
you can add a more specific comment.

```yaml
# pint disable labels/conflict($prometheus)
```

Where `$prometheus` is the name of Prometheus server to disable.

Example:

```yaml
# pint disable labels/conflict(prod)
```

## How to snooze it

You can disable this check until given time by adding a comment to it. Example:

```yaml
# pint snooze $TIMESTAMP labels/conflict
```

Where `$TIMESTAMP` is either use [RFC3339](https://www.rfc-editor.org/rfc/rfc3339)
formatted  or `YYYY-MM-DD`.
Adding this comment will disable `labels/conflict` *until* `$TIMESTAMP`, after that
check will be re-enabled.
