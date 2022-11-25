---
layout: default
parent: Checks
grand_parent: Documentation
---

# labels/conflict

This check will look for any conflicting labels used in rules.
Below is the list of conflicts it looks for.

## External labels

If recording rules are manually setting some lables that are
already present in `external_labels` Prometheus configuration option
then both labels might conflict when metrics are federated or when sending
alerts.

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
  disabled = ["labels/conflict"]
}
```

You can also disable it for all rules inside given file by adding
a comment anywhere in that file. Example:

`# pint file/disable labels/conflict`

Or you can disable it per rule by adding a comment to it. Example:

`# pint disable labels/conflict`

If you want to disable only individual instances of this check
you can add a more specific comment.

`# pint disable labels/conflict($prometheus)`

Where `$prometheus` is the name of Prometheus server to disable.

Example:

`# pint disable labels/conflict(prod)`
