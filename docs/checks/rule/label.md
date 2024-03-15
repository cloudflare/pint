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
  comment  = "..."
  severity = "bug|warning|info"
  token    = "(.*)"
  value    = "(.*)"
  values   = ["...", ...]
  required = true|false
}
```

- `$pattern` - regexp pattern to match label name on, this can be templated
  to reference checked rule fields, see [Configuration](../../configuration.md)
  for details.
- `comment` - set a custom comment that will be added to reported problems.
- `severity` - set custom severity for reported issues, defaults to a warning.
- `token` - optional regexp to tokenize label value before validating it.
  By default the whole label value is validated against `value` regexp or
  the `values` list. If you want to break the value into sub-strings and
  validate each of them independently you can do this by setting `token`
  to a regexp that captures a single sub-string.
- `value` - optional value regexp to enforce, if not set only pint will only
  check if the label exists.
- `values` - optional list of allowed values - this is alternative to using
  `value` regexp. Set this to the list of all possible valid label values.
- `required` - if `true` pint will require every rule to have this label set,
  if `false` it will only check values where label is set.

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
    comment  = "You must set a `severity` label on all alert rules"
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

If you have a label that can contain multiple different values as a single string,
for example `components: "db api memcached"`, and you want to ensure only valid values
are included then use `token` and `values`.
By setting `token` to a regexp that matches only a sequence of letters (`[a-zA-Z]+`)
you tell pint to split `"db api memcached"` into `["db", "api", "memcached"]`.
Then it iterates this list and checks each element independently.
This allows you to have validation for multi-value strings.

{% raw %}

```js
rule {
  label "components" {
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
  disabled = ["rule/label"]
}
```

You can also disable it for all rules inside given file by adding
a comment anywhere in that file. Example:

```yaml
# pint file/disable rule/label
```

Or you can disable it per rule by adding a comment to it. Example:

```yaml
# pint disable rule/label
```

If you want to disable only individual instances of this check
you can add a more specific comment.

```yaml
# pint disable rule/label($label:$required)
```

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

```yaml
# pint disable rule/label(severity:true)
```

## How to snooze it

You can disable this check until given time by adding a comment to it. Example:

```yaml
# pint snooze $TIMESTAMP rule/label
```

Where `$TIMESTAMP` is either use [RFC3339](https://www.rfc-editor.org/rfc/rfc3339)
formatted  or `YYYY-MM-DD`.
Adding this comment will disable `rule/label` *until* `$TIMESTAMP`, after that
check will be re-enabled.
