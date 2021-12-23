package config

type AlertsSettings struct {
	Range   string `hcl:"range" json:"range"`
	Step    string `hcl:"step" json:"step"`
	Resolve string `hcl:"resolve" json:"resolve"`
}

func (as AlertsSettings) validate() error {
	if as.Range != "" {
		if _, err := parseDuration(as.Range); err != nil {
			return err
		}
	}
	if as.Step != "" {
		if _, err := parseDuration(as.Step); err != nil {
			return err
		}
	}
	if as.Resolve != "" {
		if _, err := parseDuration(as.Resolve); err != nil {
			return err
		}
	}
	return nil
}
