---
layout: default
parent: Checks
grand_parent: Documentation
---

# query/cost

This check is used to calculate cost of a query and optionally report an issue
if that cost is too high. It will run `expr` query from every rule against
selected Prometheus servers and report results.
This check can be used for both recording and alerting rules, but is mostly
useful for recording rules.

## Query evaluation duration

The total duration of a query comes from Prometheus query stats included
in the API response when `?stats=1` is passed.
When enabled pint can report if `evalTotalTime` is higher than configured limit,
which can be used either for informational purpose or to fail checks on queries
that are too expensive (depending on configured `severity`).

## Query evaluation samples

Similar to evaluation duration this information comes from Prometheus query stats.
There are two different stats that give us information about the number of samples
used by given query:

- `totalQueryableSamples` - the total number of samples read during the query execution.
- `peakSamples` - the max samples kept in memory during the query execution and shows
how close the query was to reach the `--query.max-samples`` limit.

In general higher `totalQueryableSamples` means that a query either reads a lot of
time series and/or queries a large time range, both translating into longer query
execution times.
Looking at `peakSamples` on the other hand can be useful to find queries that are
complex and perform some operation on a large number of time series, for example
when you run `max(...)` on a query that returns a huge number of results.

## Series returned by the query

For recording rules anything returned by the query will be saved into Prometheus
as new time series. Checking how many time series does a rule return allows us
to estimate how much extra memory will be needed.
`pint` will try to estimate the number of bytes needed per single time series
and use that to estimate the amount of memory needed to store all the time series
returned by given query.
The `bytes per time series` number is calculated using this query:

```js
avg(avg_over_time(go_memstats_alloc_bytes[2h]) / avg_over_time(prometheus_tsdb_head_series[2h]))
```

Since Go uses garbage collector total Prometheus process memory will be more than the
sum of all memory allocations, depending on many factors like memory pressure,
Go version, `GOGC` settings etc. The estimate `pint` gives you should be considered
`best case` scenario.

## Configuration

Syntax:

```js
cost {
  comment               = "..."
  severity              = "bug|warning|info"
  maxSeries             = 5000
  maxPeakSamples        = 10000
  maxTotalSamples       = 200000
  maxEvaluationDuration = "1m"
}
```

- `comment` - set a custom comment that will be added to reported problems.
- `severity` - set custom severity for reported issues, defaults to a warning.
  This is only used when query result series exceed `maxSeries` value (if set).
  If `maxSeries` is not set or when results count is below it pint will still
  report it as information.
- `maxSeries` - if set and number of results for given query exceeds this value
  it will be reported as a bug (or custom severity if `severity` is set).
- `maxPeakSamples` - setting this to a non-zero value will tell pint to report
  any query that has higher `peakSamples` values than the value configured here.
  Nothing will be reported if this option is not set.
- `maxTotalSamples` - setting this to a non-zero value will tell pint to report
  any query that has higher `totalQueryableSamples` values than the value
  configured here. Nothing will be reported if this option is not set.
- `maxEvaluationDuration` - setting this to a non-zero value will tell pint to
  report any query that has higher `evalTotalTime` values than the value
  configured here. Nothing will be reported if this option is not set.

## How to enable it

This check is not enabled by default as it requires explicit configuration
to work.
To enable it add one or more `prometheus {...}` blocks and a `rule {...}` block
with this checks config.

Examples:

All rules from files matching `rules/dev/.+` pattern will be tested against
`dev` server. Results will be reported as information regardless of results.

```js
prometheus "dev" {
  uri     = "https://prometheus-dev.example.com"
  timeout = "30s"
  include = ["rules/dev/.+"]
}

rule {
  cost {}
}
```

Fail checks if any recording rule is using more than 300000 peak samples
or if it's taking more than 30 seconds to evaluate.

```js
rule {
  match {
    kind = "recording"
  }
  cost {
    maxPeakSamples        = 300000
    maxEvaluationDuration = "30s"
    severity              = "bug"
    comment               = "This query is too expensive to run" 
  }
}
```

## How to disable it

You can disable this check globally by adding this config block:

```js
checks {
  disabled = ["query/cost"]
}
```

You can also disable it for all rules inside given file by adding
a comment anywhere in that file. Example:

```yaml
# pint file/disable query/cost
```

Or you can disable it per rule by adding a comment to it. Example:

```yaml
# pint disable query/cost
```

If you want to disable only individual instances of this check
you can add a more specific comment.

### If `maxSeries` is set

```yaml
# pint disable query/cost($prometheus:$maxSeries)
```

Where `$prometheus` is the name of Prometheus server to disable.

Example:

```yaml
# pint disable query/cost(dev:5000)
```

### If `maxSeries` is NOT set

```yaml
# pint disable query/cost($prometheus)
```

Where `$prometheus` is the name of Prometheus server to disable.

Example:

```yaml
# pint disable query/cost(dev)
```

## How to snooze it

You can disable this check until given time by adding a comment to it. Example:

```yaml
# pint snooze $TIMESTAMP query/cost
```

Where `$TIMESTAMP` is either use [RFC3339](https://www.rfc-editor.org/rfc/rfc3339)
formatted  or `YYYY-MM-DD`.
Adding this comment will disable `query/cost` *until* `$TIMESTAMP`, after that
check will be re-enabled.
