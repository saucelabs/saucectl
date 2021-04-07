package credentials

import (
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"net/http"
	"os"
	"testing"
)

func TestEnvPrioritary(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

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
	httpmock.RegisterResponder(http.MethodGet, "https://saucelabs.com/rest/v1/users/envUsername", httpmock.NewStringResponder(200, ""))
	httpmock.RegisterResponder(http.MethodGet, "https://saucelabs.com/rest/v1/users/fileUsername", httpmock.NewStringResponder(200, ""))

	// Test No file No env
	envCreds := FromEnv()
	fileCreds := FromFile()
	overallCreds := Get()
	assert.Nil(t, envCreds)
	assert.Nil(t, fileCreds)
	assert.Nil(t, overallCreds)

	// Test Only env
	os.Setenv("SAUCE_USERNAME", "envUsername")
	os.Setenv("SAUCE_ACCESS_KEY", "envAccessKey")
	envCreds = FromEnv()
	fileCreds = FromFile()
	overallCreds = Get()
	assert.Nil(t, fileCreds)
	assert.NotNil(t, envCreds)
	assert.NotNil(t, overallCreds)
	assert.Equal(t, envCreds.Username, "envUsername")
	assert.Equal(t, envCreds.AccessKey, "envAccessKey")
	assert.Equal(t, overallCreds.Username, "envUsername")
	assert.Equal(t, overallCreds.AccessKey, "envAccessKey")

	tmpDir := os.TempDir()
	os.Setenv("USERPROFILE", tmpDir)
	os.Setenv("HOME", tmpDir)
	toSaveCreds := Credentials{
		Username: "fileUsername",
		AccessKey: "fileAccessKey",
	}
	err := toSaveCreds.Store()
	assert.Nil(t, err)

	// Removes .sauce folder from temp folder
	defer func() {
		credentialsTmpDir, _ := getCredentialsFolderPath()
		os.RemoveAll(credentialsTmpDir)
	}()

	// Test File & Env
	envCreds = FromEnv()
	fileCreds = FromFile()
	overallCreds = Get()
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
	envCreds = FromEnv()
	fileCreds = FromFile()
	overallCreds = Get()
	assert.NotNil(t, fileCreds)
	assert.Nil(t, envCreds)
	assert.NotNil(t, overallCreds)

	assert.Equal(t, fileCreds.Username, "fileUsername")
	assert.Equal(t, fileCreds.AccessKey, "fileAccessKey")
	assert.Equal(t, overallCreds.Username, "fileUsername")
	assert.Equal(t, overallCreds.AccessKey, "fileAccessKey")
}

func TestCredentials_IsValid(t *testing.T) {
	t.SkipNow() // FIXME skipping valid check until the method at test has been fixed
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(http.MethodGet, "https://saucelabs.com/rest/v1/users/validUser", httpmock.NewStringResponder(200, ""))
	validCreds := Credentials{
		Username: "validUser",
		AccessKey: "validAccessKey",
	}
	assert.True(t, validCreds.IsValid())

	httpmock.RegisterResponder(http.MethodGet, "https://saucelabs.com/rest/v1/users/invalidUser", httpmock.NewStringResponder(401, ""))
	invalidCreds := Credentials{
		Username: "invalidUser",
		AccessKey: "invalidAccessKey",
	}
	assert.False(t, invalidCreds.IsValid())

}
