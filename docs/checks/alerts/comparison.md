---
layout: default
parent: Checks
grand_parent: Documentation
---

# alerts/comparison

This check enforces use of a comparison operator in alert queries.
Any result returned by alerting rule query will trigger an alert, so it's
recommended for all alert queries to have some condition `errors > 10`, so we
only get `errors` series if the value is above 10.
If we would remove `> 10` part query would always return `errors` and so it
would always trigger an alert, even when `errors` value is `0`.

In some cases time series or specific label values will only be exported to
Prometheus under some conditions, for example `http_responses_total{code="504"}`
might only be exported if there's at least one 504 error observed.
This means that an alert with a query `http_responses_total{code="504"}` might
work perfectly fine, since in practice we'll never have this specific time
series in Prometheus with zero value.
But this setup is fragile and unpredictable, so it's highly recommended to
have always set conditions on alerts, especially that adding `> 0` shouldn't
have any negative side effects.

## Configuration

This check doesn't have any configuration options.

## How to enable it

This check is enabled by default.

## How to disable it

You can disable this check globally by adding this config block:

```js
checks {
  disabled = ["alerts/comparison"]
}
```

You can also disable it for all rules inside given file by adding
a comment anywhere in that file. Example:

`# pint file/disable alerts/comparison`

Or you can disable it per rule by adding a comment to it. Example:

`# pint disable alerts/comparison`

## How to snooze it

You can disable this check until given time by adding a comment to it. Example:

`# pint snooze $TIMESTAMP alerts/comparison`

Where `$TIMESTAMP` is either use [RFC3339](https://www.rfc-editor.org/rfc/rfc3339)
formatted  or `YYYY-MM-DD`.
Adding this comment will disable `alerts/comparison` *until* `$TIMESTAMP`, after that
check will be re-enabled.
