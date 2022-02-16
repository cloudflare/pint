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

- `$pattern` - regexp pattern to reject
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

Example:

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

## How to disable it

You can disable this check globally by adding this config block:

```js
checks {
  disabled = ["rule/reject"]
}
```

You can disable this check globally by adding this config block:

```js
checks {
  disabled = ["rule/reject"]
}
```

Or you can disable it per rule by adding a comment to it.

`# pint disable rule/reject`

If you want to disable only individual instances of this check
you can add a more specific comment.

### If `label_keys` or `annotation_keys` is set

`# pint disable rule/reject(key=~'$pattern`)`

Example:

`# pint disable rule/reject(key=~'^https?://.+$')`

### If `label_values` or `annotation_values` is set

`# pint disable promql/aggregate($label:false)`

Example:

`# pint disable rule/reject(val=~'^https?://.+$')`
