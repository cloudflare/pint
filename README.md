# pint

pint is a Prometheus rule linter.

## Usage

There are two modes it works in:
- CI PR linting
- Ad-hoc linting of a selected files or directories

### Pull Requests

It currently supports git for which it will find all commits on the current branch that are not
present in parent branch and scan all modified files included in those changes.

Results can optionally be reported using
[BitBucket API](https://docs.atlassian.com/bitbucket-server/rest/7.8.0/bitbucket-code-insights-rest.html)
to generate a report with any found issues.
Each issue will create an inline annotation in BitBucket with a description of
the issue. Exit code will always be zero when this is used, the report itself will indicate
if checks passed or not.

### Ad-hoc

Lint specified files and report any found issue.

You can lint selected files:

```SHELL
pint lint rules.yml
```

or directories:


```SHELL
pint lint path/to/dir
```

or both:


```SHELL
pint lint path/to/dir file.yml path/file.yml path/dir
```

## Quick start

Requirements:
- [Git](https://git-scm.com/)
- [Go](https://golang.org/) >=1.16

1. Build the binary:

    ```SHELL
    git clone https://github.com/cloudflare/pint.git
    cd pint
    make build
    ```

2. Run a simple syntax check on Prometheus
   [alerting](https://prometheus.io/docs/prometheus/latest/configuration/alerting_rules/)
   or [recording](https://prometheus.io/docs/prometheus/latest/configuration/recording_rules/)
   rules file(s).

    ```SHELL
    ./pint lint /etc/prometheus/*.rules.yml
    ```

3. Configuration file is optional, but without it pint will only run very basic
   syntax checks. See [CONFIGURATION.md](/docs/CONFIGURATION.md) for details on
   config syntax. Check [examples](/docs/examples) dir for sample config files.
   By default pint will try to load configuration from `.pint.hcl`, you can
   specify a different path using `--config` flag: 

    ```SHELL
    ./pint --config /etc/pint.hcl lint /etc/prometheus/rules/*.yml
    ```
