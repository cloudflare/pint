---
layout: default
parent: Checks
grand_parent: Documentation
---

# promql/range_query

This check inspects range query selectors on all queries.
It will warn if a query tries to request a time range that
is bigger than Prometheus retention limits.

By default Prometheus keeps [15 days of data](https://prometheus.io/docs/prometheus/latest/storage/#operational-aspects),
this can be customised by setting time or disk space limits.
There are two main ways of configuring retention limits in Prometheus:

- time based - Prometheus will keep last N days of metrics
- disk based - Prometheus will try to use up to N bytes of disk space.

Pint will ignore any disk space limits, since that doesn't tell us
what the effective time retention is.
But it will check the value of `--storage.tsdb.retention.time` flag passed
to Prometheus and it will warn if any selector tries to query more
data then Prometheus can store.

For example if Prometheus is running with `--storage.tsdb.retention.time=30d`
then it will store up to 30 days of historical metrics data.
If we would try to query `foo[40d]` then that query can only return up
to 30 days of data, it will never return more.

This usually isn't really a problem but can indicate a mismatch between
expectations of data retention and reality, and so you might think that by
getting results of a `avg_over_time(foo[40d])` you are getting the average
value of `foo` in the last 40 days, but in reality you're only getting
an average value in the last 30 days, and you cannot get any more than that.

You can also configure your own maximum allowed range duration if you want
to ensure that all queries are never requesting more than allowed range.
This can be done by adding a configuration rule as below.

## Configuration

Syntax:

```js
range_query {
  max        = "2h"
  comment    = "..."
  severity   = "bug|warning|info"
}
```

- `max` - duration for the maximum allowed query range.
- `comment` - set a custom comment that will be added to reported problems.
- `severity` - set custom severity for reported issues, defaults to `warning`.

## How to enable it

This check is enabled by default for all configured Prometheus servers and will
validate that queries don't use ranges longer than configured Prometheus retention.

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

Additionally you can configure an extra rule that will enforce a custom maximum
query range duration:

```js
rule {
  range_query {
    max        = "4h"
    comment    = "You cannot use range queries with range more than 4h"
    severity   = "bug"
  }
}
```

## How to disable it

You can disable this check globally by adding this config block:

```js
checks {
  disabled = ["promql/range_query"]
}
```

You can also disable it for all rules inside given file by adding
a comment anywhere in that file. Example:

```yaml
# pint file/disable promql/range_query
```

Or you can disable it per rule by adding a comment to it. Example:

```yaml
# pint disable promql/range_query
```

If you want to disable only individual instances of this check
you can add a more specific comment.

```yaml
# pint disable promql/range_query($prometheus)
```

Where `$prometheus` is the name of Prometheus server to disable.

Example:

```yaml
# pint disable promql/range_query(prod)
```

To disable a custom maximum range duration rule use:

```yaml
# pint disable promql/range_query($duration)
```

Where `$duration` is the value of `max` option in `range_query` rule.

Example:

```yaml
# pint disable promql/range_query(4h)
```

## How to snooze it

You can disable this check until given time by adding a comment to it. Example:

```yaml
# pint snooze $TIMESTAMP promql/range_query
```

Where `$TIMESTAMP` is either use [RFC3339](https://www.rfc-editor.org/rfc/rfc3339)
formatted or `YYYY-MM-DD`.
Adding this comment will disable `promql/range_query` _until_ `$TIMESTAMP`, after that
check will be re-enabled.
