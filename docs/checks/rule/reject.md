---
layout: default
parent: Checks
grand_parent: Documentation
---

# rule/reject

This check allows rejecting label or annotations keys and values using regexp
rules.

## Configuration

Syntax:

```js
reject "$pattern" {
  severity          = "bug|warning|info"
  label_keys        = true|false
  label_values      = true|false
  annotation_keys   = true|false
  annotation_values = true|false
}
```

- `$pattern` - regexp pattern to reject, this can be templated
  to reference checked rule fields, see [Configuration](../../configuration.md)
  for details
- `severity` - set custom severity for reported issues, defaults to a bug.
- `label_keys` - if true label keys for recording and alerting rules will
  be checked.
- `label_values` - if true label values for recording and alerting rules will
  be checked.
- `annotation_keys` - if true annotation keys for alerting rules will be checked.
- `annotation_values` - if true label values for alerting rules will be checked.

## How to enable it

This check is not enabled by default as it requires explicit configuration
to work.
To enable it add one or more `rule {...}` blocks and specify all rejected patterns
there.

Examples:

Disallow using URLs as label keys or values:

```js
rule {
  match {
    kind = "alerting"
  }

  reject "https?://.+" {
    label_keys = true
    label_values = true
  }
}
```

Disallow spaces in label and annotation keys:

```js
rule {
  reject ".* +.*" {
    annotation_keys = true
    label_keys = true
  }
}
```

Disallow label and annotation values equal to alert name:

{% raw %}
```js
rule {
  match {
    kind = "alerting"
  }

  reject "{{ $alert }}" {
    annotation_values = true
    label_values = true
  }
}
```
{% endraw %}

## How to disable it

You can disable this check globally by adding this config block:

```js
checks {
  disabled = ["rule/reject"]
}
```

You can also disable it for all rules inside given file by adding
a comment anywhere in that file. Example:

```yaml
# pint file/disable rule/reject
```

Or you can disable it per rule by adding a comment to it. Example:

```yaml
# pint disable rule/reject
```

If you want to disable only individual instances of this check
you can add a more specific comment.

### If `label_keys` or `annotation_keys` is set

```yaml
# pint disable rule/reject(key=~'$pattern`)
```

Example:

```yaml
# pint disable rule/reject(key=~'^https?://.+$')
```

### If `label_values` or `annotation_values` is set

```yaml
# pint disable promql/reject(val=~'$pattern')
```

Example:

```yaml
# pint disable rule/reject(val=~'^https?://.+$')
```

## How to snooze it

You can disable this check until given time by adding a comment to it. Example:

```yaml
# pint snooze $TIMESTAMP rule/reject
```

Where `$TIMESTAMP` is either use [RFC3339](https://www.rfc-editor.org/rfc/rfc3339)
formatted  or `YYYY-MM-DD`.
Adding this comment will disable `rule/reject` *until* `$TIMESTAMP`, after that
check will be re-enabled.
