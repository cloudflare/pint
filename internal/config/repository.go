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
	return nil
}

type GitHub struct {
	BaseURI   *string `hcl:"baseuri"`
	UploadURI *string `hcl:"uploaduri"`
	Timeout   string  `hcl:"timeout"`
	Owner     string  `hcl:"owner"`
	Repo      string  `hcl:"repo"`
}

func (gh GitHub) validate() error {
	if _, err := parseDuration(gh.Timeout); err != nil {
		return err
	}
	if gh.Repo == "" {
		return fmt.Errorf("repo is empty")
	}
	if gh.Owner == "" {
		return fmt.Errorf("owner is empty")
	}
	if gh.BaseURI != nil && *gh.BaseURI != "" {
		_, err := url.Parse(*gh.BaseURI)
		if err != nil {
			return fmt.Errorf("parsing baseuri: %w", err)
		}
	}
	if gh.UploadURI != nil && *gh.UploadURI != "" {
		_, err := url.Parse(*gh.UploadURI)
		if err != nil {
			return fmt.Errorf("parsing baseuri: %w", err)
		}
	}

	return nil
}

type Repository struct {
	BitBucket *BitBucket `hcl:"bitbucket,block"`
	GitHub    *GitHub    `hcl:"github,block"`
}
