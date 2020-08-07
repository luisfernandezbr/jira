package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReverseString(t *testing.T) {
	assert := assert.New(t)
	s := "hello world"
	assert.Equal("dlrow olleh", reverseString(s))
}
