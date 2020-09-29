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

func TestRemove(t *testing.T) {
	assert := assert.New(t)
	vals := []string{
		"1",
		"2",
		"a",
		"q",
	}
	assert.EqualValues([]string{"1", "a"}, removeKeys(vals, []string{"2", "q"}))
}
