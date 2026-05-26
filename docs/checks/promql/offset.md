---
layout: default
parent: Checks
grand_parent: Documentation
---

# promql/offset

This check inspects all selectors in queries for `offset` usage.
It will warn if a selector uses an offset that is larger than the
Prometheus retention period, since such a query will never return any data.

By default Prometheus keeps [15 days of data](https://prometheus.io/docs/prometheus/latest/storage/#operational-aspects),
this can be customised by setting time or disk space limits.
There are two main ways of configuring retention limits in Prometheus:

- time based - Prometheus will keep last N days of metrics
- disk based - Prometheus will try to use up to N bytes of disk space.

Pint will ignore any disk space limits, since that doesn't tell us
what the effective time retention is.
But it will check the value of `--storage.tsdb.retention.time` flag passed
to Prometheus and it will warn if any selector uses an offset larger
than Prometheus retention.

For example if Prometheus is running with `--storage.tsdb.retention.time=5d`
and we have a query like `foo offset 10d`, that query will never return
any results because Prometheus doesn't have any data that old.

This is especially problematic in binary expressions like
`foo + foo offset 10d` where one side returns data but the other
doesn't, which can lead to silently broken queries.

## Configuration

This check doesn't have any configuration options.
It is enabled automatically for all configured Prometheus servers.

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
  disabled = ["promql/offset"]
}
```

You can also disable it for all rules inside a given file by adding
a comment anywhere in that file. Example:

```yaml
# pint file/disable promql/offset
```

Or you can disable it per rule by adding a comment to it. Example:

```yaml
# pint disable promql/offset
```

If you want to disable only individual instances of this check
you can add a more specific comment.

```yaml
# pint disable promql/offset($prometheus)
```

Where `$prometheus` is the name of Prometheus server to disable.

Example:

```yaml
# pint disable promql/offset(prod)
```

## How to snooze it

You can disable this check until a given time by adding a comment to it. Example:

```yaml
# pint snooze $TIMESTAMP promql/offset
```

Where `$TIMESTAMP` is either [RFC3339](https://www.rfc-editor.org/rfc/rfc3339)
formatted or `YYYY-MM-DD`.
Adding this comment will disable `promql/offset` _until_ `$TIMESTAMP`, after which
the check will be re-enabled.
