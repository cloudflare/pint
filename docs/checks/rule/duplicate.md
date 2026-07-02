---
layout: default
parent: Checks
grand_parent: Documentation
---

# rule/duplicate

This check will find and report duplicated recording and alerting rules.

When Prometheus is configured with two identical recording rules that
are producing the exact time series it will discard results from one
of them. When that happens Prometheus will log a warning counting
discarded samples, example:

```text
msg="Error on ingesting results from rule evaluation with different value but same timestamp" num_dropped=...
```

Duplicated rule itself is not catastrophic but it will cause constant unnecessary
logs that might hide other issues and can lead to other problems if the
duplicated rule is later updated, but only in one place, not in both.

The same problem applies to alerting rules. Two alerting rules with the
same name and labels produce the same `ALERTS` time series, so when both
fire Prometheus will discard results from one of them and log the warning
above. This check reports two kinds of duplicated alerting rules:

- rules with an identical query, which always produce the same alerts,
- rules with different queries that use the same time series and can return
  the same results. For example when one query is a less specific version of the other:

  ```yaml
  - alert: Job_Running_For_Too_Long
    expr: job_duration > 5400
  - alert: Job_Running_For_Too_Long
    expr: job_duration{type!="foo"} > 3600
  ```

  Both rules can match the same series and, when they do, produce two
  identical alerts, which can be confusing.

This check only compares rules that are loaded by the same Prometheus server,
based on its `include` and `exclude` configuration. If the same rule is deployed
to two different Prometheus servers it may or may not produce duplicates based on
`external_labels` and `alert_relabel_configs` configured for each instance.
This check only warns about cases where it's clear from the rule itself that
it overlaps with another rule.

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
  disabled = ["rule/duplicate"]
}
```

You can also disable it for all rules inside a given file by adding
a comment anywhere in that file. Example:

```yaml
# pint file/disable rule/duplicate
```

Or you can disable it per rule by adding a comment to it. Example:

```yaml
# pint disable rule/duplicate
```

If you want to disable only individual instances of this check
you can add a more specific comment.

```yaml
# pint disable rule/duplicate($prometheus)
```

Where `$prometheus` is the name of Prometheus server to disable.

Example:

```yaml
# pint disable rule/duplicate(prod)
```

## How to snooze it

You can disable this check until a given time by adding a comment to it. Example:

```yaml
# pint snooze $TIMESTAMP rule/duplicate
```

Where `$TIMESTAMP` is either [RFC3339](https://www.rfc-editor.org/rfc/rfc3339)
formatted or `YYYY-MM-DD`.
Adding this comment will disable `rule/duplicate` *until* `$TIMESTAMP`, after which
the check will be re-enabled.
