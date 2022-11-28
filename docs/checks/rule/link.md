---
layout: default
parent: Checks
grand_parent: Documentation
---

# rule/link

This check allows to validate if URLs passed in alerting rule
annotations. It will send a GET request to each link matching
configured regexp and warn if it gets a non-200 response.

## Configuration

Syntax:

```js
link "$pattern" {
  severity = "bug|warning|info"
  uri      = "..."
  timeout  = "1m"
  headers  = { ... }
}
```

- `$pattern` - regexp pattern to match, each rule will only check link
  URLs that match this patter. This can be templated to reference checked
  rule fields, see [Configuration](../../configuration.md) for details.
- `uri` - optional URI rewrite rule, this will be used as the URI for all
  requests if specified, regexp capture groups can be referenced here.
- `severity` - set custom severity for reported issues, defaults to a bug.
- `timeout` - timeout to be used for all request, defaults to 1 minute.
- `headers` - a list of HTTP headers to set on all requests.

## How to enable it

This check is not enabled by default as it requires explicit configuration
to work.
To enable it add one or more `rule {...}` blocks and specify all rejected patterns
there.

Examples:

Validate all links found in alert annotations:

```js
rule {
  link "https?://.+" {}
}
```

Only validate HTTPS links, ignore plain HTTP.

```js
rule {
  link "https://.+" {}
}
```

Only validate specific hostname.

```js
rule {
  link "https?://runbooks.example.com/.+" {}
}
```

Rewrite URI to use `http://docs.internal.example.com/foo.html` instead of
`https://docs.example.com/foo.html`:

```js
rule {
  link "https?://docs.example.com/(.+)" {
    uri = "http://docs.internal.example.com/$1"
  }
}
```

Set `X-Auth` header for all requests.

```js
rule {
  link "https?://runbooks.example.com/.+" {
    headers = {
      X-Auth = "secret key"
    }
  }
}
```

## How to disable it

You can disable this check globally by adding this config block:

```js
checks {
  disabled = ["rule/link"]
}
```

You can also disable it for all rules inside given file by adding
a comment anywhere in that file. Example:

`# pint file/disable rule/link`

Or you can disable it per rule by adding a comment to it. Example:

`# pint disable rule/link`

If you want to disable only individual instances of this check
you can add a more specific comment.

`# pint disable rule/link($pattern)`

Example:

`# pint disable rule/link(^https?://.+$)`

## How to snooze it

You can disable this check until given time by adding a comment to it. Example:

`# pint snooze $TIMESTAMP rule/link`

Where `$TIMESTAMP` is either use [RFC3339](https://www.rfc-editor.org/rfc/rfc3339)
formatted  or `YYYY-MM-DD`.
Adding this comment will disable `rule/link` *until* `$TIMESTAMP`, after that
check will be re-enabled.
