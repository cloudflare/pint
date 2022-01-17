package config

import (
	"fmt"
	"net/url"
)

type BitBucket struct {
	URI        string `hcl:"uri"`
	Timeout    string `hcl:"timeout"`
	Project    string `hcl:"project"`
	Repository string `hcl:"repository"`
}

func (bb BitBucket) validate() error {
	if _, err := parseDuration(bb.Timeout); err != nil {
		return err
	}
	if bb.Project == "" {
		return fmt.Errorf("project cannot be empty")
	}
	if bb.Repository == "" {
		return fmt.Errorf("repository cannot be empty")
	}
	if bb.URI == "" {
		return fmt.Errorf("uri cannot be empty")
	}
	return nil
}

type GitHub struct {
	BaseURI   string `hcl:"baseuri,optional"`
	UploadURI string `hcl:"uploaduri,optional"`
	Timeout   string `hcl:"timeout"`
	Owner     string `hcl:"owner"`
	Repo      string `hcl:"repo"`
}

func (gh GitHub) validate() error {
	if _, err := parseDuration(gh.Timeout); err != nil {
		return err
	}
	if gh.Repo == "" {
		return fmt.Errorf("repo cannot be empty")
	}
	if gh.Owner == "" {
		return fmt.Errorf("owner cannot be empty")
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

	return nil
}

type Repository struct {
	BitBucket *BitBucket `hcl:"bitbucket,block" json:"bitbucket,omitempty"`
	GitHub    *GitHub    `hcl:"github,block" json:"github,omitempty"`
}
