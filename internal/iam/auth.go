package iam

// Credentials holds the credentials for accessing Sauce Labs.
type Credentials struct {
	Username  string                `yaml:"username"`
	AccessKey string                `yaml:"accessKey"`
	Regions   []RegionalCredentials `yaml:"regions,omitempty"`
	Source    string                `yaml:"-"`
}

// RegionalCredentials holds the credentials for accessing Sauce Labs.
type RegionalCredentials struct {
	Username  string `yaml:"username"`
	AccessKey string `yaml:"accessKey"`
	Region    string `yaml:"region"`
}

// IsSet checks whether the credentials, i.e. username and access key are not empty.
// Returns false if even one of the credentials is empty.
func (c *Credentials) IsSet() bool {
	return c.AccessKey != "" && c.Username != ""
}
