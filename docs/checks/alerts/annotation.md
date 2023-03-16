---
layout: default
parent: Checks
grand_parent: Documentation
---

# alerts/annotation

This check can be used to enforce annotations on alerting rules.

## Configuration

Syntax:

```js
annotation "$pattern" {
  severity = "bug|warning|info"
  value    = "(.*)"
  required = true|false
}
```

- `$pattern` - regexp pattern to match annotation name on, this can be templated
  to reference checked rule fields, see [Configuration](../../configuration.md)
  for details
- `severity` - set custom severity for reported issues, defaults to a warning
- `value` - optional value pattern to enforce, if not set only the
- `required` - if `true` pint will require every alert to have this annotation set,
  if `false` it will only check values where annotation is set

## How to enable it

This check is not enabled by default as it requires explicit configuration
to work.
To enable it add one or more `rule {...}` blocks and specify all required
annotations there.

Example set of rules that will:

- require `summary` annotation to be present, if missing it will be reported as a warning
- if a `dashboard` annotation is provided it must match `https://grafana\.example\.com/.+`
  pattern, if it doesn't match that pattern it will be reported as a bug

```js
rule {
  match {
    kind = "alerting"
  }

  annotation "summary" {
    required = true
  }

  annotation "dashboard" {
    severity = "bug"
    value    = "https://grafana\.example\.com/.+"
  }
}
```

Example that enforces all alerting rules with non-zero `for` field to have an
annotation called `alert_for` and value equal to `for` field.

{% raw %}

```js
rule {
  match {
    for = "> 0"
  }

  annotation "alert_for" {
    required = true
    value    = "{{ $for }}"
  }
}
```

{% endraw %}

## How to disable it

You can disable this check globally by adding this config block:

```js
checks {
  disabled = ["alerts/annotations"]
}
```

You can also disable it for all rules inside given file by adding
a comment anywhere in that file. Example:

```yaml
# pint file/disable alerts/annotation
```

Or you can disable it per rule by adding a comment to it. Example:

```yaml
# pint disable alerts/annotation
```

If you want to disable only individual instances of this check
you can add a more specific comment.

### If `value` is NOT set

```yaml
groups:
  - name: ...
    rules:
    # pint disable alerts/annotation($pattern:$required)
    - record: ...
      expr: ...
```

Example rule:

```js
annotation "summary" {
  required = true
}
```

Example comment disabling that rule:

```yaml
# pint disable alerts/annotation(summary:true)
```

### If `value` is set

```yaml
groups:
  - name: ...
    rules:
    # pint disable alerts/annotation($pattern:$value:$required)
    - record: ...
      expr: ...
```

Example rule:

```js
annotation "dashboard" {
  severity = "bug"
  value    = "https://grafana\.example\.com/.+"
}
```

Example comment disabling that rule:

```yaml
# pint disable alerts/annotation(dashboard:https://grafana\.example\.com/.+:true)
```

## How to snooze it

You can disable this check until given time by adding a comment to it. Example:

```yaml
# pint snooze $TIMESTAMP alerts/annotation
```

Where `$TIMESTAMP` is either use [RFC3339](https://www.rfc-editor.org/rfc/rfc3339)
formatted  or `YYYY-MM-DD`.
Adding this comment will disable `alerts/annotation` *until* `$TIMESTAMP`, after that
check will be re-enabled.
