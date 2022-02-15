---
layout: default
parent: Checks
grand_parent: Documentation
---

## rule/label

This check works the same way as `annotation` check, but it operates on
labels instead.
It uses static labels set on alerting or recording rule. It doesn't use
labels on time series used in those rules.

Syntax:

```js
label "(.*)" {
  severity = "bug|warning|info"
  value    = "..."
  required = true|false
}
```

Example:

Require `severity` label to be set on alert rules with two all possible values:

```js
rule {
  match {
    kind = "alerting"
  }

  label "severity" {
    value    = "(warning|critical)"
    required = true
  }
}
```
