package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

type BitBucket struct {
	URI         string `hcl:"uri"`
	Timeout     string `hcl:"timeout,optional"`
	Project     string `hcl:"project"`
	Repository  string `hcl:"repository"`
	MaxComments int    `hcl:"maxComments,optional"`
}

func (bb BitBucket) validate() error {
	if _, err := parseDuration(bb.Timeout); err != nil {
		return err
	}
	if bb.Project == "" {
		return errors.New("project cannot be empty")
	}
	if bb.Repository == "" {
		return errors.New("repository cannot be empty")
	}
	if bb.URI == "" {
		return errors.New("uri cannot be empty")
	}
	if bb.MaxComments < 0 {
		return errors.New("maxComments cannot be negative")
	}
	return nil
}

type GitHub struct {
	BaseURI     string `hcl:"baseuri,optional"`
	UploadURI   string `hcl:"uploaduri,optional"`
	Timeout     string `hcl:"timeout,optional"`
	Owner       string `hcl:"owner,optional"`
	Repo        string `hcl:"repo,optional"`
	MaxComments int    `hcl:"maxComments,optional"`
}

func (gh GitHub) validate() error {
	if repo := os.Getenv("GITHUB_REPOSITORY"); repo != "" {
		parts := strings.SplitN(repo, "/", 2)
		if len(parts) != 2 {
			return fmt.Errorf(
				"GITHUB_REPOSITORY is set, but with an invalid repository format: %s",
				repo,
			)
		}
		if gh.Repo == "" && parts[1] == "" {
			return errors.New("repo cannot be empty")
		}
		if gh.Owner == "" && parts[0] == "" {
			return errors.New("owner cannot be empty")
		}
	} else {
		if gh.Repo == "" {
			return errors.New("repo cannot be empty")
		}
		if gh.Owner == "" {
			return errors.New("owner cannot be empty")
		}
	}
	if _, err := parseDuration(gh.Timeout); err != nil {
		return err
	}
	if gh.BaseURI != "" {
		_, err := url.Parse(gh.BaseURI)
		if err != nil {
			return fmt.Errorf("invalid baseuri: %w", err)
		}
	}
	if gh.UploadURI != "" {
		_, err := url.Parse(gh.UploadURI)
		if err != nil {
			return fmt.Errorf("invalid uploaduri: %w", err)
		}
	}
	if gh.MaxComments < 0 {
		return errors.New("maxComments cannot be negative")
	}
	return nil
}

type GitLab struct {
	URI         string `hcl:"uri,optional"`
	Timeout     string `hcl:"timeout,optional"`
	Project     int    `hcl:"project"`
	MaxComments int    `hcl:"maxComments,optional"`
}

func (gl GitLab) validate() error {
	if gl.Project <= 0 {
		return errors.New("project must be set")
	}
	if gl.MaxComments < 0 {
		return errors.New("maxComments cannot be negative")
	}
	return nil
}

type Repository struct {
	BitBucket *BitBucket `hcl:"bitbucket,block" json:"bitbucket,omitempty"`
	GitHub    *GitHub    `hcl:"github,block"    json:"github,omitempty"`
	GitLab    *GitLab    `hcl:"gitlab,block"    json:"gitlab,omitempty"`
}

func (r *Repository) validate() (err error) {
	if r.BitBucket != nil {
		if r.BitBucket.Timeout == "" {
			r.BitBucket.Timeout = time.Minute.String()
		}
		if r.BitBucket.MaxComments == 0 {
			r.BitBucket.MaxComments = 50
		}
		if err = r.BitBucket.validate(); err != nil {
			return err
		}
	}

	if r.GitHub != nil {
		if r.GitHub.Timeout == "" {
			r.GitHub.Timeout = time.Minute.String()
		}
		if r.GitHub.MaxComments == 0 {
			r.GitHub.MaxComments = 50
		}
		if err = r.GitHub.validate(); err != nil {
			return err
		}
	}

	if r.GitLab != nil {
		if r.GitLab.Timeout == "" {
			r.GitLab.Timeout = time.Minute.String()
		}
		if r.GitLab.MaxComments == 0 {
			r.GitLab.MaxComments = 50
		}
		if err = r.GitLab.validate(); err != nil {
			return err
		}
	}

	return nil
}
