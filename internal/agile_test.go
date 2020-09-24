package internal

import (
	"testing"
	"time"

	"github.com/pinpt/agent.next/sdk"
	"github.com/stretchr/testify/assert"
)

func TestSprintStatusMap(t *testing.T) {
	assert := assert.New(t)
	assert.Equal(sdk.AgileSprintStatusFuture, sprintStateMap["future"])
	assert.Equal(sdk.AgileSprintStatusActive, sprintStateMap["active"])
	assert.Equal(sdk.AgileSprintStatusClosed, sprintStateMap["closed"])
}

func TestMakeSprintUpdateName(t *testing.T) {
	assert := assert.New(t)
	evt := sdk.AgileSprintUpdateMutation{}
	evt.Set.Name = sdk.StringPointer("my sprint 1")
	update, updated, err := makeSprintUpdate("5", &evt)
	assert.NoError(err)
	assert.True(updated)
	assert.Equal("{\"id\":5,\"name\":\"my sprint 1\"}", sdk.Stringify(update))
}

func TestMakeSprintUpdateGoal(t *testing.T) {
	assert := assert.New(t)
	evt := sdk.AgileSprintUpdateMutation{}
	evt.Set.Goal = sdk.StringPointer("get things done")
	update, updated, err := makeSprintUpdate("5", &evt)
	assert.NoError(err)
	assert.True(updated)
	assert.Equal("{\"id\":5,\"goal\":\"get things done\"}", sdk.Stringify(update))
}

func TestMakeSprintUpdateStartDate(t *testing.T) {
	assert := assert.New(t)
	evt := sdk.AgileSprintUpdateMutation{}
	ts, err := time.Parse("2006-01-02", "2020-09-22")
	assert.NoError(err)
	evt.Set.StartDate = &ts
	update, updated, err := makeSprintUpdate("5", &evt)
	assert.NoError(err)
	assert.True(updated)
	assert.Equal("{\"id\":5,\"startDate\":\"2020-09-22T00:00:00Z\"}", sdk.Stringify(update))
}

func TestMakeSprintUpdateEndDate(t *testing.T) {
	assert := assert.New(t)
	evt := sdk.AgileSprintUpdateMutation{}
	ts, err := time.Parse("2006-01-02", "2020-09-22")
	assert.NoError(err)
	evt.Set.EndDate = &ts
	update, updated, err := makeSprintUpdate("5", &evt)
	assert.NoError(err)
	assert.True(updated)
	assert.Equal("{\"id\":5,\"endDate\":\"2020-09-22T00:00:00Z\"}", sdk.Stringify(update))
}

func TestMakeSprintUpdateCompletedDate(t *testing.T) {
	assert := assert.New(t)
	evt := sdk.AgileSprintUpdateMutation{}
	ts, err := time.Parse("2006-01-02", "2020-09-22")
	assert.NoError(err)
	evt.Set.CompletedDate = &ts
	update, updated, err := makeSprintUpdate("5", &evt)
	assert.NoError(err)
	assert.True(updated)
	assert.Equal("{\"id\":5,\"completeDate\":\"2020-09-22T00:00:00Z\"}", sdk.Stringify(update))
}

func TestMakeSprintUpdateStatus(t *testing.T) {
	assert := assert.New(t)
	evt := sdk.AgileSprintUpdateMutation{}
	v := sdk.AgileSprintStatusClosed
	evt.Set.Status = &v
	update, updated, err := makeSprintUpdate("5", &evt)
	assert.NoError(err)
	assert.True(updated)
	assert.Equal("{\"id\":5,\"state\":\"closed\"}", sdk.Stringify(update))
}
