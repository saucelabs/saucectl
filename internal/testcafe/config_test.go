package testcafe

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSetDefaultValues (t *testing.T) {
	s := Suite{
		Speed: 0,
		SelectorTimeout: 0,
		AssertionTimeout: 0,
		PageLoadTimeout: 0,
	}
	setDefaultValues(&s)
	assert.Equal(t, s.Speed, float64(1))
	assert.Equal(t, s.SelectorTimeout, 10000)
	assert.Equal(t, s.AssertionTimeout, 3000)
	assert.Equal(t, s.PageLoadTimeout, 3000)

	s = Suite{
		Speed: 2,
	}
	setDefaultValues(&s)
	assert.Equal(t, s.Speed, float64(1))

	s = Suite{
		Speed: 0.5,
	}
	setDefaultValues(&s)
	assert.Equal(t, s.Speed, 0.5)

	s = Suite{
		Speed: 0,
		SelectorTimeout: -1,
		AssertionTimeout: -1,
		PageLoadTimeout: -1,
	}
	setDefaultValues(&s)
	assert.Equal(t, s.Speed, float64(1))
	assert.Equal(t, s.SelectorTimeout, 10000)
	assert.Equal(t, s.AssertionTimeout, 3000)
	assert.Equal(t, s.PageLoadTimeout, 3000)
}