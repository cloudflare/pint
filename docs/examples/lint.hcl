# This is an example config to be used when running pint as a linter

lint {
  # Check all files inside rules/alerting and rules/recording dirs.
  include    = ["rules/(alerting|recording)/.+"]

  # Ignore all *.md and *.txt files.
  exclude    = [".+.md", ".+.txt"]
}
