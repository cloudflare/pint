# Example BitBucket repository configuration for CI pull request comments.
# When running 'pint ci' with this config, problems will be posted
# back to the PR as comments on BitBucket.
# Requires BITBUCKET_AUTH_TOKEN environment variable to be set.

repository {
  bitbucket {
    # Base URI of the BitBucket server.
    uri = "https://bitbucket.example.com"

    # BitBucket project key.
    project = "MYPROJ"

    # BitBucket repository name.
    repository = "my-repo"

    # Limit the number of comments pint creates on a single PR.
    maxComments = 50
  }
}
