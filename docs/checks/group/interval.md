---
layout: default
parent: Checks
grand_parent: Documentation
---

# group/interval

This check will warn when a rule group declares an `interval` value greater
than 5 minutes.

Prometheus evaluates rules based on the group `interval` and each evaluation
becomes a sample for the resulting time series of a recording rule or a
potential alert state change for an alerting rule.

If you set the group `interval` to a value greater than 5 minutes you will
end up with gaps in the recording rule results when querying them, because
Prometheus defaults to a 5 minute query lookback. See the
[staleness](https://prometheus.io/docs/prometheus/latest/querying/basics/#staleness)
section of the Prometheus documentation for details.

Any sample older than 5 minutes is considered stale and won't be returned by
instant queries, so a recording rule that only produces a sample every
10 minutes will appear to be missing for half of that time when queried.

For alerting rules, a long interval will also cause flapping alerts, because
the alert state is only re-evaluated every `interval`. Between evaluations
the alert can resolve and then fire again on the next tick, unless you also
set `keep_firing_for` to a value greater than or equal to the group
`interval`.

Example rule group that will trigger this check:

```yaml
groups:
- name: example
  interval: 10m
  rules:
  - record: job:up:sum
    expr: sum(up) by(job)
```

To fix this problem, either lower the `interval` to `5m` or less, or, for
alerting rules, set `keep_firing_for` to a value greater than or equal to
the group `interval`.

## Configuration

This check doesn't have any configuration options.

## How to enable it

This check is enabled by default.

## How to disable it

You can disable this check globally by adding this config block:

```js
checks {
  disabled = ["group/interval"]
}
```

You can also disable it for all rules inside given file by adding
a comment anywhere in that file. Example:

```yaml
# pint file/disable group/interval
```

Or you can disable it per rule by adding a comment to it. Example:

```yaml
# pint disable group/interval
```

## How to snooze it

You can disable this check until given time by adding a comment to it. Example:

```yaml
# pint snooze $TIMESTAMP group/interval
```

Where `$TIMESTAMP` is either use [RFC3339](https://www.rfc-editor.org/rfc/rfc3339)
formatted  or `YYYY-MM-DD`.
Adding this comment will disable `group/interval` *until* `$TIMESTAMP`, after that
check will be re-enabled.
