---
layout: default
parent: Checks
grand_parent: Documentation
---

# promql/performance

This check will try to find rules using queries that are being precomputed using recording rules.
Consider these rules:

```yaml
- record: foo:rate5m
  expr: rate(foo_total[5m])

- alert: Rate Too High
  expr: sum(rate(foo_total[5m])) without(instance) > 10
```

Here we have an alert `Rate Too High` that uses `rate(foo_total[5m])` as part of the query.
We also have a recording rule `foo:rate5m` that calculates the same expression and stores it
as a metric.
Instead of calculating `rate(foo_total[5m])` in both rules we can simply query `foo:rate5m` inside
`Rate Too High` alert to speed it up.

`promql/performance` will try to find cases like this and emit an information report for it.

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
  disabled = ["promql/performance"]
}
```

You can also disable it for all rules inside given file by adding
a comment anywhere in that file. Example:

```yaml
# pint file/disable promql/performance
```

Or you can disable it per rule by adding a comment to it. Example:

```yaml
# pint disable promql/performance
```

If you want to disable only individual instances of this check
you can add a more specific comment.

```yaml
# pint disable promql/performance($prometheus)
```

Where `$prometheus` is the name of Prometheus server to disable.

Example:

```yaml
# pint disable promql/performance(prod)
```

## How to snooze it

You can disable this check until given time by adding a comment to it. Example:

```yaml
# pint snooze $TIMESTAMP promql/performance
```

Where `$TIMESTAMP` is either use [RFC3339](https://www.rfc-editor.org/rfc/rfc3339)
formatted  or `YYYY-MM-DD`.
Adding this comment will disable `promql/performance` *until* `$TIMESTAMP`, after that
check will be re-enabled.
