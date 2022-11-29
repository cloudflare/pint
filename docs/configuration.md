---
layout: default
title: Configuration
parent: Documentation
nav_order: 2
---

# Configuration syntax

## Table of contents
{: .no_toc .text-delta }

1. TOC
{:toc}

## Regexp matchers

All regexp patterns use [Go regexp](https://pkg.go.dev/regexp) module and are fully anchored.
This means that when you pass `.*` regexp expression internally it will be represented as
`^.*$`, where `^` indicates beginning of a string and `$` is the end of string.
This follows [PromQL behavior](https://prometheus.io/docs/prometheus/latest/querying/basics/)
for consistency with Prometheus.
If you have a string `alice bob john` and you want to match a substring `bob`, then be sure to use
`.*bob.*`.

When using regexp matcher in checks configuration you can reference alerting and recording rule
fields in the regexp using [Go text/template](https://pkg.go.dev/text/template) syntax.
Rule fields are exposed as:

- `$alert` - rule `alert` field
- `$record` - rule `record` field
- `$expr` - rule `expr` field
- `$for` - rule `for` field
- `$labels` - rule `labels` map, individual labels can be accessed as `$labels.foo`
- `$annotations` - rule `annotations` map, individual annotations can be accessed as `$annotations.foo`

Accessing a field that's not present in the rule will return an empty string.

## Parser

Configure how pint parses Prometheus rule files.

Syntax:

```js
parser {
  relaxed = [ "(.*)", ... ]
}
```

- `relaxed` - by default pint will now parse all files in strict mode, where
  all rule files must have the exact syntax Prometheus expects:

  ```yaml
  groups:
  - name: example
    rules:
    - record: ...
      expr: ...
  ```

  If you're using pint to lint rules that are embedded inside a different structure
  you can set this option to allow fuzzy parsing, which will try to find rule
  definitions anywhere in the file, without requiring `groups -> rules -> rule`
  structure to be present.
  This option takes a list of file patterns, all files matching those regexp rules
  will be parsed in relaxed mode.

## CI

Configure continuous integration environments.

Syntax:

```js
ci {
  include    = [ "(.*)", ... ]
  maxCommits = 20
  baseBranch = "master"
}
```

- `include` - list of file patterns to check when running checks. Only files
  matching those regexp rules will be checked, other modified files will be ignored.
- `maxCommits` - by default pint will try to find all commits on the current branch,
  this requires full git history to be present, if we have a shallow clone this
  might fail to find only current branch commits and give us a huge list.
  If the number of commits returned by branch discovery is more than `maxCommits`
  then pint will fail to run.
- `baseBranch` - base branch to compare `HEAD` commit with when calculating the list
  of commits to check.

## Repository

Configure supported code hosting repository, used for reporting PR checks from CI
back to the repository, to be displayed in the PR UI.
Currently it only supports [BitBucket](https://bitbucket.org/) and [GitHub](https://github.com/).

**NOTE**: BitBucket integration requires `BITBUCKET_AUTH_TOKEN` environment variable
to be set. It should contain a personal access token used to authenticate with the API.

**NOTE**: GitHub integration requires `GITHUB_AUTH_TOKEN` environment variable
to be set to a personal access key that can access your repository.

**NOTE** Pull request number must be known to pint so it can add comments if it detects any problems.
If pint is run as part of GitHub actions workflow then this number will be detected from `GITHUB_REF`
environment variable. For other use cases `GITHUB_PULL_REQUEST_NUMBER` environment variable must be set
with the pull request number.

Syntax:

```js
repository {
  bitbucket {
    uri        = "https://..."
    timeout    = "1m"
    project    = "..."
    repository = "..."
  }
}
```

- `bitbucket:uri` - base URI of this repository, will be used for HTTP
  requests to the BitBucket API.
- `bitbucket:timeout` - timeout to be used for API requests, defaults to 1 minute.
- `bitbucket:project` - name of the BitBucket project for this repository.
- `bitbucket:repository` - name of the BitBucket repository.

```js
repository {
  github {
    baseuri    = "https://..."
    uploaduri  = "https://..."
    timeout    = "1m"
    owner      = "..."
    repo       = "..."
  }
}
```

- `github:baseuri` - base URI of GitHub or GitHub enterprise, will be used for HTTP requests to the GitHub API.
  If not set `pint` will try to use `GITHUB_API_URL` environment variable instead (if set).
- `github:uploaduri` - upload URI of GitHub or GitHub enterprise, will be used for HTTP requests to the GitHub API.
  If not set `pint` will try to use `GITHUB_API_URL` environment variable instead (if set).

If `github:baseuri` _or_ `github:uploaduri` are not specified then [GitHub](https://github.com) will be used.

- `github:timeout` - timeout to be used for API requests, defaults to 1 minute.
- `github:owner` - name of the GitHub owner i.e. the first part that comes before the repository's name in the URI.
  If not set `pint` will try to use `GITHUB_REPOSITORY` environment variable instead (if set).
- `github:repo` - name of the GitHub repository (e.g. `monitoring`).
  If not set `pint` will try to use `GITHUB_REPOSITORY` environment variable instead (if set).

Most GitHub settings can be detected from environment variables that are set inside GitHub Actions
environment. The only exception is `GITHUB_AUTH_TOKEN` environment variable that must be set
manually.

## Prometheus servers

Some checks work by querying a running Prometheus instance to verify if
metrics used in rules are present. If you want to use those checks then you
first need to define one or more Prometheus servers.

Syntax:

```js
prometheus "$name" {
  uri         = "https://..."
  failover    = ["https://...", ...]
  headers     = { "...": "..." }
  timeout     = "2m"
  concurrency = 16
  rateLimit   = 100
  required    = true|false
  include     = ["...", ...]
  exclude     = ["...", ...]
}
```

- `$name` - each defined server should have a unique name that can be used in check
  definitions.
- `uri` - base URI of this Prometheus server, used for API requests and queries.
- `failover` - list of URIs to try (in order they are specified) if `uri` doesn't respond
  to requests or returns an error. This allows to configure failover Prometheus servers
  to avoid CI failures in case main Prometheus server is unreachable.
  Failover URIs are not used if Prometheus returns an error caused by the query, like
  `many-to-many matching not allowed`.
  It's highly recommended that all URIs point to Prometheus servers with identical
  configuration, otherwise pint checks might return unreliable results and potential
  false positives.
- `headers` - a list of HTTP headers that will be set on all requests for this Prometheus
  server.
- `timeout` - timeout to be used for API requests. Defaults to 2 minutes.
- `concurrency` - how many concurrent requests can pint send to this Prometheus server.
  Optional, defaults to 16.
- `rateLimit` - per second rate limit for all API requests send to this Prometheus server.
  Setting it to `1000` would allow for up to 1000 requests per each wall clock second.
  Optional, default to 100 requests per second.
- `uptime` - metric selector used to detect gaps in Prometheus uptime.
  Since some checks are sending queries to validate if given metric always present in Prometheus
  they might find gaps when Prometheus itself was down. Pint tries to detect that by querying
  metrics that are always guarnateed to be present when Prometheus is running.
  By default metric used for this is `up`, which is generated by Prometheus itself, see
  [Prometheus docs](https://prometheus.io/docs/concepts/jobs_instances/#automatically-generated-labels-and-time-series)
  for details.
  Uptime gap detection works by running a range query `count(up)`  and checking for any gaps
  in the response.
  Since `up` metric can have a lot of time series `count(up)` might be slow and expensive.
  An alternative is to use one of metrics exposed by Prometheus itself, like `prometheus_build_info`, but
  those metrics are only present if Prometheus is configured to scrape itself, so `up` is used by default
  since it's guaranteed to work in every setup.
  If your Prometheus has a lot of time series and it's configured to scrape itself then
  it is recommeded to set `uptime` field to `prometheus_build_info`.
- `required` - decides how pint will report errors if it's unable to get a valid response
  from this Prometheus server. If `required` is `true` and all API calls to this Prometheus
  fail pint will report those as `bug` level problem. If it's set to `false` pint will
  report those with `warning` level.
  Default value for `required` is `false`. Set it to `true` if you want to hard fail
  in case of remote Prometheus issues. Note that setting it to `true` might block
  PRs when running `pint ci` until pint is able to talk to Prometheus again.
- `include` - optional path filter, if specified only paths matching one of listed regexp
  patterns will use this Prometheus server for checks.
- `exclude` - optional path filter, if specified any path matching one of listed regexp
  patterns will never use this Prometheus server for checks.
  `exclude` takes precedence over `include.

Example:

```js
prometheus "prod" {
  uri         = "https://prometheus-prod.example.com"
  headers     = {
    "X-Auth": "secret"
  }
  concurrency = 40
}

prometheus "staging" {
  uri    = "https://prometheus-staging.example.com"
  uptime = "prometheus_build_info"
}

prometheus "dev" {
  uri     = "https://prometheus-dev.example.com"
  timeout = "30s"
  include = [ "alerts/test/.*" ]
  exclude = [ "alerts/test/docs/.*" ]
}
```

## Matching rules to checks

Most checks, except basic syntax verification, requires some configuration to decide
which checks to run against which files and rules.

Syntax:

```js
rule {
  match {
    path = "(.+)"
    name = "(.+)"
    kind = "alerting|recording"
    command = "ci|lint|watch"
    annotation "(.*)" {
      value = "(.*)"
    }
    label "(.*)" {
      value = "(.*)"
    }
    for = "..."
  }
  match { ... }
  match { ... }
  ignore {
    path = "(.+)"
    name = "(.+)"
    kind = "alerting|recording"
    command = "ci|lint|watch"
    annotation "(.*)" {
      value = "(.*)"
    }
    label "(.*)" {
      value = "(.*)"
    }
    for = "..."
  }
  ignore { ... }
  ignore { ... }

  [ check definition ]
  ...
  [ check definition ]
}
```

- `match:path` - only files matching this pattern will be checked by this rule
- `match:name` - only rules with names (`record` for recording rules and `alert` for alerting
  rules) matching this pattern will be checked rule
- `match:kind` - optional rule type filter, only rule of this type will be checked
- `match:command` - optional command type filter, this allows to include or ignore rules
  based on the command pint is run with `pint ci`, `pint lint` or `pint watch`.
- `match:annotation` - optional annotation filter, only alert rules with at least one
  annotation matching this pattern will be checked by this rule.
- `match:label` - optional annotation filter, only rules with at least one label
   matching this pattern will be checked by this rule. For recording rules only static
   labels set on the recording rule are considered.
- `match:for` - optional alerting rule `for` filter. If set only alerting rules with `for`
  field present and matching provided value will be checked by this rule. Recording rules
  will never match it as they don't have `for` field.
  Syntax is `OP DURATION` where `OP` can be any of `=`, `!=`, `>`, `>=`, `<`, `<=`.
- `ignore` - works exactly like `match` but does the opposite - any alerting or recording rule
  matching all conditions defined on `ignore` will not be checked by this `rule` block.

Note: both `match` and `ignore` require all defined filters to be satisfied to work.
If multiple `match` and/or `ignore` rules are present any of them needs to match for the rule to
be matched / ignored.

Examples:

```js
rule {
  match {
    path = "rules/.*"
    kind = "alerting"
    label "severity" {
      value = "(warning|critical)"
    }
  }
  ignore {
    command = "watch"
  }
  [ check applied only to severity="critical" and severity="warning" alerts in "ci" or "lint" command is run ]
}
```

```js
rule {
  ignore {
    command = "watch"
  }
  ignore {
    command = "lint"
  }
  [ check applied unless "watch" or "lint" command is run ]
}
```

```js
rule {
  match {
    for = ">= 5m"
  }
  [ check applied only to alerting rules with "for" field value that is >= 5m ]
}
```
