package iam

// Credentials contains a set of Username + AccessKey for SauceLabs.
type Credentials struct {
	Username  string `yaml:"username"`
	AccessKey string `yaml:"accessKey"`
}

// IsEmpty checks whether the credentials, i.e. username and access key are not empty.
// Returns false if even one of the credentials is empty.
func (c *Credentials) IsEmpty() bool {
	return c.AccessKey == "" || c.Username == ""
}

// IsValid validates that the credentials are valid.
func (c *Credentials) IsValid() bool {
	return !c.IsEmpty()
}
