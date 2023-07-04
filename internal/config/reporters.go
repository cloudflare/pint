package config

type Reporters struct {
	JSON *JSONReporterSettings `hcl:"json,block" json:"json,omitempty"`
}
