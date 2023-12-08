# This is an example config to be used when running pint as a CI job
# to validate pull requests.

ci {
  # Check all files inside rules/alerting and rules/recording dirs.
  include    = ["rules/(alerting|recording)/.+"]

  # Ignore all *.md and *.txt files.
  exclude    = [".+.md", ".+.txt"]

  # Don't run pint if there are more than 50 commits on current branch.
  maxCommits = 50

  # When running 'pint ci' compare current branch with origin/main
  # to get the list of modified files.
  baseBranch = "origin/main"
}
