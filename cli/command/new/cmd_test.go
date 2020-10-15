package new

import (
	"github.com/saucelabs/saucectl/cli/config"
	"gotest.tools/assert"
	"os"
	"testing"
)

func Test_updateRegion(t *testing.T) {
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