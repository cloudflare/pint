---
layout: default
parent: Checks
grand_parent: Documentation
---

# rule/dependency

This check validates rule dependencies in two scenarios:

## Removed dependencies

When running `pint ci`, this check validates that any removed recording rule
isn't still being used by other rules.

Removing any recording rule that is a dependency of other rules is likely
to make them stop working, unless there's some other source of the metric
that was produced by the removed rule.

Example: consider these two rules, one generates `down:count` metric
that is then used by the alert rule:

```yaml
groups:
- name: ...
  rules:
  - record: down:count
    expr: count(up == 0) by(job)
  - alert: Job is down
    expr: down:count > 0
```

If we were to edit this file and delete the recording rule:

```yaml
groups:
- name: ...
  rules:
  - alert: Job is down
    expr: down:count > 0
```

This would leave our alert rule broken because Prometheus would
no longer have `down:count` metric.

This check tries to detect scenarios like this but works across all
files.

## Cross-group dependencies

This check warns when a rule uses a metric produced by a recording rule
in a different group.

Each group is evaluated independently with no guaranteed execution order.
During evaluation, recording rules write results to a transaction that is
committed to TSDB at the end of the group run. Queries within the same group
can see uncommitted results from earlier rules in that group, but queries in
other groups can only see values that have been committed to TSDB.

In practice, if rule B depends on rule A, they should be in the same group.
Otherwise, B will see a stale value of A from the previous evaluation cycle,
adding up to one evaluation interval of lag.

Example:

```yaml
groups:
- name: recordings
  rules:
  - record: error:rate5m
    expr: rate(errors_total[5m])

- name: alerts
  rules:
  - alert: HighErrorRate
    expr: error:rate5m > 0.1
```

The `HighErrorRate` alert uses `error:rate5m` from a different group.
Both groups run at fixed intervals but start at different times.
With a one-minute interval:

- **0s** - `recordings` evaluates `error:rate5m` and commits to TSDB.
- **55s** - `alerts` queries `error:rate5m` and sees a 55-second-old value.
- **60s** - `recordings` runs again.
- **115s** - `alerts` runs again, still seeing stale data.

Each time `alerts` runs, it sees a value that is already up to one interval old.

Moving both rules to the same group eliminates this lag:

```yaml
groups:
- name: errors
  rules:
  - record: error:rate5m
    expr: rate(errors_total[5m])
  - alert: HighErrorRate
    expr: error:rate5m > 0.1
```

- **0s** - `errors` evaluates `error:rate5m` and writes to the transaction.
- **0s** - `errors` evaluates `HighErrorRate`, which sees the value from the
  previous step with no lag.

Now the alert always sees the freshly computed value.

## Configuration

This check doesn't have any configuration options.

## How to enable it

This check is enabled by default.

## How to disable it

You can disable this check globally by adding this config block:

```js
checks {
  disabled = ["rule/dependency"]
}
```

You can also disable it for all rules inside given file by adding
a comment anywhere in that file. Example:

```yaml
# pint file/disable rule/dependency
```

Or you can disable it per rule by adding a comment to it. Example:

```yaml
# pint disable rule/dependency
```

## How to snooze it

You can disable this check until given time by adding a comment to it. Example:

```yaml
# pint snooze $TIMESTAMP rule/dependency
```

Where `$TIMESTAMP` is either use [RFC3339](https://www.rfc-editor.org/rfc/rfc3339)
formatted  or `YYYY-MM-DD`.
Adding this comment will disable `rule/dependency` *until* `$TIMESTAMP`, after that
check will be re-enabled.
