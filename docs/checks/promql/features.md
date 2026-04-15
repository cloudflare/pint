---
layout: default
parent: Checks
grand_parent: Documentation
---

# promql/features

This check verifies that PromQL queries don't use features that require
`--enable-feature=...` flags that are not enabled on the Prometheus server
this rule would be deployed to.

Some PromQL features are experimental and must be explicitly enabled on
the Prometheus server. If a rule uses such features but the target Prometheus
server doesn't have the required feature flags enabled, the query will fail
at evaluation time.

Pint will query the [flags API](https://prometheus.io/docs/prometheus/latest/querying/api/#flags)
of each configured Prometheus server to get the list of enabled feature flags
and compare it against features required by the query.
It will also query the [build info API](https://prometheus.io/docs/prometheus/latest/querying/api/#build-information)
to determine the running Prometheus version and report when a feature
requires a newer version than the one running.

Currently detected features:

- `promql-experimental-functions` - required when using experimental PromQL functions
  such as `mad_over_time`, `sort_by_label`, `sort_by_label_desc`, `info`,
  `double_exponential_smoothing`, `limitk`, `limit_ratio`, and others.
- `promql-duration-expr` - required when using arithmetic expressions in time durations.
- `promql-extended-range-selectors` - required when using `anchored` or `smoothed` range selector modifiers.
- `promql-binop-fill-modifiers` - required when using `fill()` binary operator modifier.

See [Prometheus feature flags documentation](https://prometheus.io/docs/prometheus/latest/feature_flags/)
for details on each feature.

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
  disabled = ["promql/features"]
}
```

You can also disable it for all rules inside given file by adding
a comment anywhere in that file. Example:

```yaml
# pint file/disable promql/features
```

Or you can disable it per rule by adding a comment to it. Example:

```yaml
# pint disable promql/features
```

If you want to disable only individual instances of this check
you can add a more specific comment.

```yaml
# pint disable promql/features($prometheus)
```

Where `$prometheus` is the name of Prometheus server to disable.

Example:

```yaml
# pint disable promql/features(prod)
```

## How to snooze it

You can disable this check until given time by adding a comment to it. Example:

```yaml
# pint snooze $TIMESTAMP promql/features
```

Where `$TIMESTAMP` is either use [RFC3339](https://www.rfc-editor.org/rfc/rfc3339)
formatted or `YYYY-MM-DD`.
Adding this comment will disable `promql/features` *until* `$TIMESTAMP`, after that
check will be re-enabled.
