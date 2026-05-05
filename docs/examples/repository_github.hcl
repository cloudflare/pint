# Example GitHub repository configuration for CI pull request comments.
# When running 'pint ci' with this config, problems will be posted
# back to the PR as comments on GitHub.
# Most settings can be auto-detected from GITHUB_* env vars.

repository {
  github {
    # Base URI for GitHub Enterprise; omit for github.com.
    baseuri = "https://github.example.com"

    # Repository owner and name.
    owner = "my-org"
    repo  = "my-repo"

    # Limit the number of comments pint creates on a single PR.
    maxComments = 50
  }
}
