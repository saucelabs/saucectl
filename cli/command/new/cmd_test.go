package new

import (
	"github.com/saucelabs/saucectl/internal/config"
	"gotest.tools/assert"
	"gotest.tools/v3/fs"
	"os"
	"testing"
)

func TestUpdateRegion(t *testing.T) {
	cfgFile := "./test-config.yml"
	fd, err := os.Create(cfgFile)
	assert.NilError(t, err)
	fd.Close()

	c, err := config.NewJobConfiguration(cfgFile)
	assert.NilError(t, err)
	assert.Equal(t, c.Sauce.Region, "")

	err = updateRegion(cfgFile, "us-west-1")
	assert.NilError(t, err)

	c, err = config.NewJobConfiguration(cfgFile)
	assert.NilError(t, err)
	assert.Equal(t, c.Sauce.Region, "us-west-1")

	err = os.Remove(cfgFile)
	assert.NilError(t, err)
}

func TestUpdateRegionCypress(t *testing.T) {
	dir := fs.NewDir(t, "fixtures",
		fs.WithFile("config.yml", "apiVersion: v1alpha\nkind: cypress\ncypress:\n  configFile: cypress.json\n  version: 1.2.3", fs.WithMode(0644)),
		fs.WithFile("cypress.json", "{}", fs.WithMode(0644)),
		fs.WithDir("cypress", fs.WithMode(0755)))
	path, _ := os.Getwd()
	defer func() {
		os.Chdir(path)
		dir.Remove()
	}()
	assert.Equal(t, nil, os.Chdir(dir.Path()))
	assert.Equal(t, nil, updateRegion("config.yml", "eu-central-1"))
	c, err := config.NewJobConfiguration("config.yml")
	assert.Equal(t, nil, err)
	assert.Equal(t, "eu-central-1", c.Sauce.Region)
}

func TestUpdateRegionPlaywright(t *testing.T) {
	dir := fs.NewDir(t, "fixtures",
		fs.WithFile("config.yml", "apiVersion: v1alpha\nkind: playwright\nplaywright:\n  projectPath: dummy-folder\n  version: 1.2.3", fs.WithMode(0644)))
	path, _ := os.Getwd()
	defer func() {
		os.Chdir(path)
		dir.Remove()
	}()
	assert.Equal(t, nil, os.Chdir(dir.Path()))
	assert.Equal(t, nil, updateRegion("config.yml", "eu-central-1"))
	c, err := config.NewJobConfiguration("config.yml")
	assert.Equal(t, nil, err)
	assert.Equal(t, "eu-central-1", c.Sauce.Region)
}
