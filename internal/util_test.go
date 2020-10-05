package internal

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/pinpt/agent/v4/sdk"
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

func TestGetJiraErrorMessage(t *testing.T) {
	assert := assert.New(t)
	netErr := sdk.HTTPError{
		StatusCode: http.StatusBadRequest,
		Body:       bytes.NewBuffer([]byte(`{"errorMessages":[],"errors":{"summary":"Field 'summary' cannot be set. It is not on the appropriate screen, or unknown.","description":"Field 'description' cannot be set. It is not on the appropriate screen, or unknown."}}`)),
	}
	errStr := getJiraErrorMessage(&netErr)
	assert.EqualValues("summary: Field 'summary' cannot be set. It is not on the appropriate screen, or unknown.", errStr)
}
