package credentials

import (
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestEnvPrioritary(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	defer func() {
		os.Remove(getCredentialsFilePath())
	}()

	// Prepare to restore home var
	userprofile := os.Getenv("USERPROFILE")
	home := os.Getenv("HOME")
	defer func() {
		os.Setenv("USERPROFILE", userprofile)
		os.Setenv("HOME", home)
	}()

	// Override Home
	os.Setenv("USERPROFILE", "C:\\non-existent")
	os.Setenv("HOME", "/tmp/non-existent")

	os.Unsetenv("SAUCE_USERNAME")
	os.Unsetenv("SAUCE_ACCESS_KEY")

	// To avoid conflict with real file
	httpmock.RegisterResponder("GET", "https://saucelabs.com/rest/v1/users/envUsername", httpmock.NewStringResponder(200, ""))
	httpmock.RegisterResponder("GET", "https://saucelabs.com/rest/v1/users/fileUsername", httpmock.NewStringResponder(200, ""))

	// Test No file No env
	envCreds := GetCredentialsFromEnv()
	fileCreds := GetCredentialsFromFile()
	overallCreds := GetCredentials()
	assert.Nil(t, envCreds)
	assert.Nil(t, fileCreds)
	assert.Nil(t, overallCreds)

	// Test Only env
	os.Setenv("SAUCE_USERNAME", "envUsername")
	os.Setenv("SAUCE_ACCESS_KEY", "envAccessKey")
	envCreds = GetCredentialsFromEnv()
	fileCreds = GetCredentialsFromFile()
	overallCreds = GetCredentials()
	assert.Nil(t, fileCreds)
	assert.NotNil(t, envCreds)
	assert.NotNil(t, overallCreds)
	assert.Equal(t, envCreds.Username, "envUsername")
	assert.Equal(t, envCreds.AccessKey, "envAccessKey")
	assert.Equal(t, overallCreds.Username, "envUsername")
	assert.Equal(t, overallCreds.AccessKey, "envAccessKey")


	wd, _ := os.Getwd()
	os.Setenv("USERPROFILE", wd)
	os.Setenv("HOME", wd)
	toSaveCreds := Credentials{
		Username: "fileUsername",
		AccessKey: "fileAccessKey",
	}
	err := toSaveCreds.Store()
	assert.Nil(t, err)

	// Test File & Env
	envCreds = GetCredentialsFromEnv()
	fileCreds = GetCredentialsFromFile()
	overallCreds = GetCredentials()
	assert.NotNil(t, fileCreds)
	assert.NotNil(t, envCreds)
	assert.NotNil(t, overallCreds)

	assert.Equal(t, envCreds.Username, "envUsername")
	assert.Equal(t, envCreds.AccessKey, "envAccessKey")
	assert.Equal(t, fileCreds.Username, "fileUsername")
	assert.Equal(t, fileCreds.AccessKey, "fileAccessKey")
	assert.Equal(t, overallCreds.Username, "envUsername")
	assert.Equal(t, overallCreds.AccessKey, "envAccessKey")

	// Test Only file
	os.Unsetenv("SAUCE_USERNAME")
	os.Unsetenv("SAUCE_ACCESS_KEY")
	envCreds = GetCredentialsFromEnv()
	fileCreds = GetCredentialsFromFile()
	overallCreds = GetCredentials()
	assert.NotNil(t, fileCreds)
	assert.Nil(t, envCreds)
	assert.NotNil(t, overallCreds)

	assert.Equal(t, fileCreds.Username, "fileUsername")
	assert.Equal(t, fileCreds.AccessKey, "fileAccessKey")
	assert.Equal(t, overallCreds.Username, "fileUsername")
	assert.Equal(t, overallCreds.AccessKey, "fileAccessKey")
}

func TestCredentials_IsValid(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	defer func() {
		os.Unsetenv("SAUCE_USERNAME")
		os.Unsetenv("SAUCE_ACCESS_KEY")
	}()

	// Prepare to restore home var
	userprofile := os.Getenv("USERPROFILE")
	home := os.Getenv("HOME")
	defer func() {
		os.Setenv("USERPROFILE", userprofile)
		os.Setenv("HOME", home)
	}()
	os.Setenv("USERPROFILE", "C:\\non-existent")
	os.Setenv("HOME", "/tmp/non-existent")

	httpmock.RegisterResponder("GET", "https://saucelabs.com/rest/v1/users/validUser", httpmock.NewStringResponder(200, ""))
	httpmock.RegisterResponder("GET", "https://saucelabs.com/rest/v1/users/invalidUser", httpmock.NewStringResponder(401, ""))

	os.Setenv("SAUCE_USERNAME", "validUser")
	os.Setenv("SAUCE_ACCESS_KEY", "validAccessKey")
	envCreds := GetCredentials()
	assert.NotNil(t, envCreds)

	os.Setenv("SAUCE_USERNAME", "invalidUser")
	os.Setenv("SAUCE_ACCESS_KEY", "invalidAccessKey")
	envCreds = GetCredentials()
	assert.Nil(t, envCreds)
}
