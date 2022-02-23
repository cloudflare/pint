---
layout: default
parent: Checks
grand_parent: Documentation
---

# rule/label

This check works the same way as [alerts/annotation](../alerts/annotation.md) check,
but it operates on labels instead.
It uses static labels set on alerting or recording rule. It doesn't use
labels on time series used in those rules.

## Configuration

Syntax:

```js
label "$pattern" {
  severity = "bug|warning|info"
  value    = "..."
  required = true|false
}
```

- `$pattern` - regexp pattern to match label name on, this can be templated
  to reference checked rule fields, see [Configuration](../../configuration.md)
  for details
- `severity` - set custom severity for reported issues, defaults to a warning
- `value` - optional value pattern to enforce, if not set only the 
- `required` - if `true` pint will require every rule to have this label set,
  if `false` it will only check values where label is set

## How to enable it

This check is not enabled by default as it requires explicit configuration
to work.
To enable it add one or more `rule {...}` blocks and specify all required
labels there.

Example that will require `severity` label to be set on alert rules with two
all possible values:

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

Example that enforces all alerting rules with `for` value present and greater
than 5 minutes field to have a label called `alert_for` and value equal to
`for` field.

{% raw %}
```js
rule {
  match {
    for = "> 5m"
  }

  label "alert_for" {
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
  disabled = ["rule/label"]
}
```

Or you can disable it per rule by adding a comment to it:

`# pint disable rule/label`

If you want to disable only individual instances of this check
you can add a more specific comment.

`# pint disable rule/label($label:$required)`

Where `$label` is the label name and `$required` is the configure value
of `required` option.

```yaml
groups:
  - name: ...
    rules:
    # pint disable rule/label($pattern:$required)
    - record: ...
      expr: ...
```

Example rule:

```js
label "severity" {
  value    = "(warning|critical)"
  required = true
}
```

Example comment disabling that rule:

`# pint disable rule/label(severity:true)`
