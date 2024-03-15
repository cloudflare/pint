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
  comment  = "..."
  severity = "bug|warning|info"
  token    = "(.*)"
  value    = "(.*)"
  values   = ["...", ...]
  required = true|false
}
```

- `$pattern` - regexp pattern to match annotation name on, this can be templated
  to reference checked rule fields, see [Configuration](../../configuration.md)
  for details.
- `comment` - set a custom comment that will be added to reported problems.
- `severity` - set custom severity for reported issues, defaults to a warning.
- `token` - optional regexp to tokenize annotation value before validating it.
  By default the whole annotation value is validated against `value` regexp or
  the `values` list. If you want to break the value into sub-strings and
  validate each of them independently you can do this by setting `token`
  to a regexp that captures a single sub-string.
- `value` - optional value regexp to enforce, if not set only pint will only
  check if the annotation exists.
- `values` - optional list of allowed values - this is alternative to using.
  `value` regexp. Set this to the list of all possible valid annotation values.
- `required` - if `true` pint will require every rule to have this annotation set,
  if `false` it will only check values where annotation is set.

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
    comment  = "You must add a link do a Grafana dashboard showing impact of this alert"
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

If you have an annotation that can contain multiple different values as a single string,
for example `components: "db api memcached"`, and you want to ensure only valid values
are included then use `token` and `values`.
By setting `token` to a regexp that matches only a sequence of letters (`[a-zA-Z]+`)
you tell pint to split `"db api memcached"` into `["db", "api", "memcached"]`.
Then it iterates this list and checks each element independently.
This allows you to have validation for multi-value strings.

{% raw %}

```js
rule {
  annotation "components" {
    required = true
    token    = "[a-zA-Z]+"
    values   = [
      "prometheus",
      "db",
      "memcached",
      "api",
      "storage",
    ]
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
