---
layout: default
parent: Checks
grand_parent: Documentation
---

# promql/rate

This check inspects `rate()` and `irate()` functions and warns if used duration
is too low. It does so by first getting global `scrape_interval` value for selected
Prometheus servers and comparing duration to it.
Reported issue depends on a few factors:

For `rate()` function:
- If duration is less than 2x `scrape_interval` it will report a bug.
- If duration is between 2x and 4x `scrape_interval` it will report a warning.

For `irate()` function:
- If duration is less than 2x `scrape_interval` it will report a bug.
- If duration is between 2x and 3x `scrape_interval` it will report a warning.

## Configuration

This check doesn't have any configuration options.

## How to enable it

This check is enabled by default for all configured Prometheus servers.

Example:

```js
prometheus "prod" {
  uri     = "https://prometheus-prod.example.com"
  timeout = "60s"
  paths = [
    "rules/prod/.*",
    "rules/common/.*",
  ]
}

prometheus "dev" {
  uri     = "https://prometheus-dev.example.com"
  timeout = "30s"
  paths = [
    "rules/dev/.*",
    "rules/common/.*",
  ]
}
```

## How to disable it

You can disable this check globally by adding this config block:

```js
checks {
  disabled = ["promql/rate"]
}
```

Or you can disable it per rule by adding a comment to it:

`# pint disable promql/rate`

If you want to disable only individual instances of this check
you can add a more specific comment.

`# pint disable promql/rate($prometheus)`

Where `$prometheus` is the name of Prometheus server to disable.

Example:

`# pint disable promql/rate(prod)`