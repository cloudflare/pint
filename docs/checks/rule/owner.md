---
layout: default
parent: Checks
grand_parent: Documentation
---

# rule/owner

This check can be used to enforce rule ownership comments used by `pint watch`
command when exporting metrics about problems detected in rules.

If you see this check reports it means that `--require-owner` flag is enabled
for pint and a rule file is missing required ownership comment.

To set a rule owner add a `# pint file/owner $owner` comment in a file, to set
an owner for all rules in that file. You can also set an owner per rule, by adding
`# pint rule/owner $owner` comment around given rule.

Example:

```yaml
# pint file/owner bob

- alert: ...
  expr: ...

# pint rule/owner alice
- alert: ...
  expr: ...
```

## Configuration

This check doesn't have any configuration options.

## How to enable it

This check is enabled only if you pass `--require-owner` flag to `pint lint`
or `pint ci` commands.

## How to disable it

Remove `--require-owner` flag from pint CLI arguments.
