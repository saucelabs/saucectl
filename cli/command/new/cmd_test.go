package new

import (
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/cypress"
	"github.com/saucelabs/saucectl/internal/playwright"
	"github.com/saucelabs/saucectl/internal/testcafe"
	"gopkg.in/yaml.v2"
	"gotest.tools/assert"
	"gotest.tools/v3/fs"
	"os"
	"testing"
)

func TestUpdateRegion(t *testing.T) {
	dir := fs.NewDir(t, "common",
		fs.WithFile("config.yml", "apiVersion: v1alpha\nsauce:\n  region: us-west-1\n  concurrency: 1\n ", fs.WithMode(0644)))
	path, _ := os.Getwd()
	defer func() {
		os.Chdir(path)
		dir.Remove()
	}()
	os.Chdir(dir.Path())
	err := updateRegion("config.yml", "eu-central-1")
	assert.NilError(t, err, "region should be updated successfully")

	type MockProject struct {
		Sauce config.SauceConfig `yaml:"sauce,omitempty" json:"sauce"`
	}

	var conf MockProject
	f, err := os.Open(dir.Join("config.yml"))
	defer f.Close()
	if err != nil {
		t.Errorf("failed to open config file: %v", err)
	}
	if err = yaml.NewDecoder(f).Decode(&conf); err != nil {
		t.Errorf("failed to parse project config: %v", err)
	}

	assert.NilError(t, err, "No error when reading file")
	assert.Equal(t, "eu-central-1", conf.Sauce.Region, "region is updated")
}

func TestUpdateRegionCypress(t *testing.T) {
	dir := fs.NewDir(t, "cypress",
		fs.WithFile("config.yml", "apiVersion: v1alpha\nkind: cypress\ncypress:\n  configFile: cypress.json\n  version: 1.2.3\nsauce:\n  region: us-west-1\n", fs.WithMode(0644)),
		fs.WithFile("cypress.json", "{}", fs.WithMode(0644)),
		fs.WithDir("cypress", fs.WithMode(0755)))
	path, _ := os.Getwd()
	defer func() {
		os.Chdir(path)
		dir.Remove()
	}()
	os.Chdir(dir.Path())
	err := updateRegion("config.yml", "eu-central-1")
	assert.NilError(t, err, "region should be updated successfully")
	c, err := cypress.FromFile("config.yml")
	assert.NilError(t, err, "No error when reading file")
	assert.Equal(t, "eu-central-1", c.Sauce.Region, "region is updated")
}

func TestUpdateRegionPlaywright(t *testing.T) {
	dir := fs.NewDir(t, "playwright",
		fs.WithFile("config.yml", "apiVersion: v1alpha\nkind: playwright\nplaywright:\n  projectPath: dummy-folder\n  version: 1.2.3\nsauce:\n  region: us-west-1\n", fs.WithMode(0644)))
	path, _ := os.Getwd()
	defer func() {
		os.Chdir(path)
		dir.Remove()
	}()
	os.Chdir(dir.Path())
	err := updateRegion("config.yml", "eu-central-1")
	assert.NilError(t, err, "region should be updated successfully")
	c, err := playwright.FromFile("config.yml")
	assert.NilError(t, err, "No error when reading file")
	assert.Equal(t, "eu-central-1", c.Sauce.Region, "region is updated")
}

func TestUpdateRegionTestCafe(t *testing.T) {
	dir := fs.NewDir(t, "testcafe",
		fs.WithFile("config.yml", "apiVersion: v1alpha\nkind: testcafe\ntestcafe:\n  projectPath: dummy-folder\n  version: 1.2.3\nsauce:\n  region: us-west-1\n", fs.WithMode(0644)))
	path, _ := os.Getwd()
	defer func() {
		os.Chdir(path)
		dir.Remove()
	}()
	os.Chdir(dir.Path())
	err := updateRegion("config.yml", "eu-central-1")
	assert.NilError(t, err, "region should be updated successfully")
	c, err := testcafe.FromFile("config.yml")
	assert.NilError(t, err, "No error when reading file")
	assert.Equal(t, "eu-central-1", c.Sauce.Region, "region is updated")
}
