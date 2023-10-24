package imagerunner

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUtils_GetCanonicalServiceName(t *testing.T) {
	assert.Equal(t, "", GetCanonicalServiceName(""))
	assert.Equal(t, "foo", GetCanonicalServiceName("foo"))
	assert.Equal(t, "fo123o", GetCanonicalServiceName("FO123o"))
	assert.Equal(t, "fo1-23o", GetCanonicalServiceName("FO1_23o"))
	assert.Equal(t, "fo1-23o", GetCanonicalServiceName(" FO1 23o "))
	assert.Equal(t, "s-23ao", GetCanonicalServiceName("_23Ao"))
}
