---
layout: default
parent: Checks
grand_parent: Documentation
---

## rule/reject

This check allows rejecting label or annotations keys and values
using regexp rules.

Syntax:

```js
reject "(.*)" {
  severity          = "bug|warning|info"
  label_keys        = true|false
  label_values      = true|false
  annotation_keys   = true|false
  annotation_values = true|false
}
```

- `severity` - set custom severity for reported issues, defaults to a bug.
- `label_keys` - if true label keys for recording and alerting rules will
  be checked.
- `label_values` - if true label values for recording and alerting rules will
  be checked.
- `annotation_keys` - if true annotation keys for alerting rules will be checked.
- `annotation_values` - if true label values for alerting rules will be checked.

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
