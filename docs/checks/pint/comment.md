---
layout: default
parent: Checks
grand_parent: Documentation
---

# pint/comment

You will only ever see this check reporting problems if you have a file
with a comment that looks like one of pint control comments
(see [page](../../ignoring.md)) but pint cannot parse it.

This might be, for example, when you're trying to disable some checks
for a specific rule using `# pint disable ...`, but the comment is not
"touching" any rule and so pint cannot apply it.

Valid rule specific comments:

```yaml
# pint disable promql/series(my_metric)
- record: foo
  expr: sum(my_metric) without(instance)

- record: foo
  # pint disable promql/series(my_metric)
  expr: sum(my_metric) without(instance)
```

Invalid comment that's not attached to any rule:

```yaml
# pint disable promql/series(my_metric)

- record: foo
  expr: sum(my_metric) without(instance)
```

## Configuration

This check doesn't have any configuration options.

## How to enable it

This check is enabled by default.

## How to disable it

You cannot disable this check.
