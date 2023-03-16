---
layout: default
parent: Checks
grand_parent: Documentation
---

# rule/duplicate

This check will find and report any duplicated recording rules.

When Prometheus is configured with two identical recording rules that
are producing the exact time series it will discard results from one
of them. When that happens you will see warnings in logs, example:

```
msg="Rule evaluation result discarded" err="duplicate sample for timestamp"
```

Duplicated rule itself is not catastrophic but it will cause constant unnecessary
logs that might hide other issues and can lead to other problems if the
duplicated rule is later updated, but only in one place, not in both.

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

You can also disable it for all rules inside given file by adding
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

You can disable this check until given time by adding a comment to it. Example:

```yaml
# pint snooze $TIMESTAMP rule/duplicate
```

Where `$TIMESTAMP` is either use [RFC3339](https://www.rfc-editor.org/rfc/rfc3339)
formatted  or `YYYY-MM-DD`.
Adding this comment will disable `rule/duplicate` *until* `$TIMESTAMP`, after that
check will be re-enabled.
