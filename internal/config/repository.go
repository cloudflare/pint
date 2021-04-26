package config

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

type Repository struct {
	BitBucket *BitBucket `hcl:"bitbucket,block"`
}
