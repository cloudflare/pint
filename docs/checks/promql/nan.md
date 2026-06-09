---
layout: default
parent: Checks
grand_parent: Documentation
---

# promql/nan

This check warns when a division or modulo operation inside an aggregation can produce
non-finite values (`NaN` or `Inf`), causing the entire aggregation result to become
non-finite.

## How non-finite values appear

Division by zero in PromQL produces non-finite values:

- `0 / 0` = `NaN`
- `1 / 0` = `+Inf`
- `-1 / 0` = `-Inf`
- `0 % 0` = `NaN`
- `1 % 0` = `NaN`

Only `0/0` produces `NaN` for division, while non-zero divided by zero produces `Inf`.
Modulo by zero always produces `NaN`.

## Why this is a problem

A division or modulo by zero inside an aggregation can introduce `NaN` or `Inf`
into the aggregation input.

For arithmetic aggregations, a single `NaN` or `Inf` input can make the final
result `NaN` or `Inf`.

Consider this recording rule:

```yaml
- record: error_ratio:sum
  expr: sum by (cluster) (errors / total)
```

And these input time series:

```text
errors{cluster="us", instance="a"} 5
errors{cluster="us", instance="b"} 3
errors{cluster="us", instance="c"} 0
total{cluster="us",  instance="a"} 100
total{cluster="us",  instance="b"} 200
total{cluster="us",  instance="c"} 0
```

The division `errors / total` produces:

```text
{cluster="us", instance="a"} = 5 / 100 = 0.05
{cluster="us", instance="b"} = 3 / 200 = 0.015
{cluster="us", instance="c"} = 0 / 0   = NaN
```

When `sum()` aggregates these three values, the result is:

```text
error_ratio:sum{cluster="us"} = 0.05 + 0.015 + NaN = NaN
```

A single `NaN` or `Inf` value in any input series makes the entire aggregation
result `NaN` or `Inf`. The two perfectly valid series (`instance="a"` and
`instance="b"`) are lost because of one bad series.

## Details

### Affected aggregations

This check warns for aggregations where non-finite values can invalidate or skew
the result:

- `sum()`
- `avg()`
- `stddev()`
- `stdvar()`

### Safe aggregations

The following aggregations are **not** affected in the same way because they
either ignore input values or don't combine them in a way that contaminates
other series:

- `count()` — counts the number of series, regardless of their values.
- `count_values()` — counts occurrences of each distinct value, including `NaN`
  and `Inf` as distinct values.
- `group()` — returns `1` for each group, ignoring values entirely.
- `quantile()` — `Inf` values are valid inputs.
- `topk()` — selects top series; `NaN` in one series doesn't affect others.
- `bottomk()` — selects bottom series; `NaN` in one series doesn't affect others.
- `limitk()` — selects a subset of series without combining values.
- `limit_ratio()` — selects a ratio of series without combining values.

## Recommended fixes

### Guard the divisor

Filter out zero-valued divisor series before division so non-finite values are never produced:

```yaml
- record: error_ratio:sum
  expr: sum(errors / (total > 0))
```

This drops series where `total` is zero before the division happens.

### Clamp the divisor

Clamp the divisor to a range that excludes zero:

```yaml
- record: error_ratio:sum
  expr: sum(errors / clamp_min(total, 1))
```

This replaces zero values in `total` with `1`. Only use this when a minimum of `1`
is acceptable for your use case.

You can also use other clamp forms as long as the output range cannot include zero:

```yaml
- record: negative_ratio:sum
  expr: sum(errors / clamp_max(total, -1))

- record: bounded_ratio:sum
  expr: sum(errors / clamp(total, 1, 10))
```

These are safe because the divisor is forced to stay strictly above zero or
strictly below zero.

## Configuration

This check doesn't have any configuration options.

## How to enable it

This check is enabled by default.

## How to disable it

You can disable this check globally by adding this config block:

```js
checks {
  disabled = ["promql/nan"]
}
```

You can also disable it for all rules inside a given file by adding
a comment anywhere in that file. Example:

```yaml
# pint file/disable promql/nan
```

Or you can disable it per rule by adding a comment to it. Example:

```yaml
# pint disable promql/nan
```

## How to snooze it

You can disable this check until a given time by adding a comment to it. Example:

```yaml
# pint snooze $TIMESTAMP promql/nan
```

Where `$TIMESTAMP` is either [RFC3339](https://www.rfc-editor.org/rfc/rfc3339)
formatted or `YYYY-MM-DD`.
Adding this comment will disable `promql/nan` *until* `$TIMESTAMP`, after which
the check will be re-enabled.
