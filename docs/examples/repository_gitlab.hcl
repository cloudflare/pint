# Example GitLab repository configuration for CI pull request comments.
# When running 'pint ci' with this config, problems will be posted
# back to the MR as comments on GitLab.
# Requires GITLAB_AUTH_TOKEN environment variable to be set.

repository {
  gitlab {
    # Optional self-hosted GitLab URI; omit for gitlab.com.
    uri = "https://gitlab.example.com"

    # GitLab project ID.
    project = "12345"

    # Limit the number of comments pint creates on a single MR.
    maxComments = 50
  }
}
