package internal

import (
	"testing"

	"github.com/pinpt/agent.next/sdk"
	"github.com/stretchr/testify/assert"
)

func TestSprintStatusMap(t *testing.T) {
	assert := assert.New(t)
	assert.Equal(sdk.AgileSprintStatusFuture, sprintStateMap["future"])
	assert.Equal(sdk.AgileSprintStatusActive, sprintStateMap["active"])
	assert.Equal(sdk.AgileSprintStatusClosed, sprintStateMap["closed"])
}
