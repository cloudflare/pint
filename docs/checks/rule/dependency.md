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

This check warns when a recording rule uses a metric produced by another
recording rule in a different group. Alerting rules are ignored by this check
because the small lag is usually acceptable.

Each group is evaluated independently with no guaranteed execution order.
Rules inside the same group run sequentially by default. As soon as a rule
finishes evaluating, its samples are committed to the TSDB. This means later
rules in the same group will see freshly written data from earlier rules.

If recording rule B depends on recording rule A but they are in different
groups, B will see a stale value of A from the previous evaluation cycle,
adding up to one evaluation interval of lag. This happens because both groups
run at fixed intervals but start at different times.

Example:

```yaml
groups:
- name: base
  rules:
  - record: error:rate5m
    expr: rate(errors_total[5m])

- name: aggregations
  rules:
  - record: error:rate5m:avg
    expr: avg(error:rate5m)
```

The `error:rate5m:avg` recording rule uses `error:rate5m` from a different group.
Both groups run at fixed intervals but start at different times.
With a one-minute interval:

- `@ 0s` - `base` evaluates `error:rate5m` and commits to TSDB.
- `@ 55s` - `aggregations` queries `error:rate5m` and sees a 55-second-old value.
- `@ 60s` - `base` runs again.
- `@ 115s` - `aggregations` runs again and sees value from `@ 60s` which is 55 seconds old.

Each time `aggregations` runs, it sees a value that is already up to one interval old.
If you query for both metrics at `@ 70s` you'll get:

- `error:rate5m` sample calculated `@ 60s` with value from `@ 60s`.
- `error:rate5m:avg` sample calculated `@ 55s` with value from `@ 0s`.

This means there will be a confusing mismatch between these two metrics.

Moving both rules to the same group eliminates this lag:

```yaml
groups:
- name: errors
  rules:
  - record: error:rate5m
    expr: rate(errors_total[5m])
  - record: error:rate5m:avg
    expr: avg(error:rate5m)
```

- `@ 0s` - `errors` evaluates `error:rate5m` and commits to TSDB.
- `@ 0s` - `errors` evaluates `error:rate5m:avg`, which sees the value from the
  previous step with no lag.
- `@ 60s` - `errors` runs again, new value for `error:rate5m` is calculated.
- `@ 60s` - `error:rate5m:avg` runs again and sees most recent value for `error:rate5m`, no lag.

Now the aggregation always sees the freshly computed value, querying both metrics at `@ 70s`
will produce results that are consistent between them and there will be no confusion due to lag.

## Configuration

This check can be configured to ignore specific metrics when checking for
 cross-group dependencies.

### `ignoreGroupMismatch`

A list of regular expressions matching metric names that should be ignored
when checking for cross-group dependencies. Metrics matching any of these
patterns won't trigger a warning when used by a recording rule from a different
group.

Example:

```js
check "rule/dependency" {
  ignoreGroupMismatch = ["foo:.*", "bar:sum"]
}
```

This is useful when you have recording rules that depend on other recording
rules in different groups, but you know the lag is acceptable for specific
metrics.

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

Where `$TIMESTAMP` is either [RFC3339](https://www.rfc-editor.org/rfc/rfc3339)
formatted or `YYYY-MM-DD`.
Adding this comment will disable `rule/dependency` *until* `$TIMESTAMP`, after that
check will be re-enabled.
