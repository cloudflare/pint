---
layout: default
parent: Checks
grand_parent: Documentation
---

# alerts/absent

This check will warn you about alerting rules that are using `absent()` calls without having `for` option set
to at least 2x scrape interval.
Using `absent()` without `for` can cause false positive alerts when Prometheus is restarted and the rule
is evaluated before the metrics tested using `absent()` are scraped. Adding a `for` option with at least
2x scrape interval is usually enough to prevent this from happening.

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
  disabled = ["alerts/absent"]
}
```

You can also disable it for all rules inside given file by adding
a comment anywhere in that file. Example:

```yaml
# pint file/disable alerts/absent
```

Or you can disable it per rule by adding a comment to it. Example:

```yaml
# pint disable alerts/absent
```

If you want to disable only individual instances of this check
you can add a more specific comment.

```yaml
# pint disable alerts/absent($prometheus)
```

Where `$prometheus` is the name of Prometheus server to disable.

Example:

```yaml
# pint disable alerts/absent(prod)
```

## How to snooze it

You can disable this check until given time by adding a comment to it. Example:

```yaml
# pint snooze $TIMESTAMP alerts/absent
```

Where `$TIMESTAMP` is either use [RFC3339](https://www.rfc-editor.org/rfc/rfc3339)
formatted  or `YYYY-MM-DD`.
Adding this comment will disable `alerts/absent` *until* `$TIMESTAMP`, after that
check will be re-enabled.
