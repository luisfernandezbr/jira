package internal

import (
	"io/ioutil"
	"testing"

	"github.com/pinpt/agent.next/sdk"
	"github.com/pinpt/agent.next/sdk/sdktest"
	"github.com/pinpt/integration-sdk/agent"
	"github.com/stretchr/testify/assert"
)

func loadFile(fn string) []byte {
	b, err := ioutil.ReadFile(fn)
	if err != nil {
		panic("error reading test data file: " + err.Error())
	}
	return b
}

func TestWebhookJiraIssueDeleted(t *testing.T) {
	assert := assert.New(t)
	pipe := &sdktest.MockPipe{}
	i := JiraIntegration{
		logger: sdk.NewNoOpTestLogger(),
	}
	assert.NoError(i.webhookDeleteIssue("1234", "1", loadFile("testdata/jira:issue_deleted.json"), pipe))
	assert.Len(pipe.Written, 1)
	update := pipe.Written[0].(*agent.UpdateData)
	assert.EqualValues("false", update.Set["active"])
}

func TestWebhookJiraIssueCommentDeleted(t *testing.T) {
	assert := assert.New(t)
	pipe := &sdktest.MockPipe{}
	i := JiraIntegration{
		logger: sdk.NewNoOpTestLogger(),
	}
	assert.NoError(i.webhookDeleteComment("1234", "1", loadFile("testdata/comment_deleted.json"), pipe))
	assert.Len(pipe.Written, 1)
	update := pipe.Written[0].(*agent.UpdateData)
	assert.EqualValues("false", update.Set["active"])
}

func TestWebhookJiraProjectDeleted(t *testing.T) {
	assert := assert.New(t)
	pipe := &sdktest.MockPipe{}
	i := JiraIntegration{
		logger: sdk.NewNoOpTestLogger(),
	}
	assert.NoError(i.webhookDeleteProject("1234", "1", loadFile("testdata/project_deleted.json"), pipe))
	assert.Len(pipe.Written, 1)
	update := pipe.Written[0].(*agent.UpdateData)
	assert.EqualValues("false", update.Set["active"])
}

func TestWebhookJiraUserCreated(t *testing.T) {
	assert := assert.New(t)
	logger := sdk.NewNoOpTestLogger()
	um := &mockUserManager{}
	assert.NoError(webhookUpsertUser(logger, um, loadFile("testdata/user_created.json")))
	assert.Len(um.users, 1)
	assert.EqualValues("jhaynie+1", um.users[0].DisplayName)
	assert.EqualValues("5f03c8345ee2c300232945de", um.users[0].AccountID)
}

func TestWebhookJiraUserUpdated(t *testing.T) {
	assert := assert.New(t)
	logger := sdk.NewNoOpTestLogger()
	um := &mockUserManager{}
	assert.NoError(webhookUpsertUser(logger, um, loadFile("testdata/user_updated.json")))
	assert.Len(um.users, 1)
	assert.EqualValues("jeff haynie test", um.users[0].DisplayName)
	assert.EqualValues("5f03c8345ee2c300232945de", um.users[0].AccountID)
}

func TestWebhookJiraSprintDeleted(t *testing.T) {
	assert := assert.New(t)
	pipe := &sdktest.MockPipe{}
	i := JiraIntegration{
		logger: sdk.NewNoOpTestLogger(),
	}
	assert.NoError(i.webhookDeleteSprint("1234", "1", loadFile("testdata/sprint_deleted.json"), pipe))
	assert.Len(pipe.Written, 1)
	update := pipe.Written[0].(*agent.UpdateData)
	assert.EqualValues("false", update.Set["active"])
}

const sprintUpdateGoalAdded = `{
  "timestamp": 1596142226397,
  "webhookEvent": "sprint_updated",
  "sprint": {
    "id": 196,
    "self": "https://pinpt-hq.atlassian.net/rest/agile/1.0/sprint/196",
    "state": "future",
    "name": "TES Sprint 2",
    "originBoardId": 9,
    "goal": "take over the world 🌍🌎🌏"
  },
  "oldValue": {
    "id": 196,
    "self": "https://pinpt-hq.atlassian.net/rest/agile/1.0/sprint/196",
    "state": "future",
    "name": "TES Sprint 2",
    "originBoardId": 9
  }
}`

const sprintUpdateGoalUpdated = `{
  "timestamp": 1596142259617,
  "webhookEvent": "sprint_updated",
  "sprint": {
    "id": 196,
    "self": "https://pinpt-hq.atlassian.net/rest/agile/1.0/sprint/196",
    "state": "future",
    "name": "TES Sprint 2",
    "originBoardId": 9,
    "goal": "take over the world! 🌍🌎🌏"
  },
  "oldValue": {
    "id": 196,
    "self": "https://pinpt-hq.atlassian.net/rest/agile/1.0/sprint/196",
    "state": "future",
    "name": "TES Sprint 2",
    "originBoardId": 9,
    "goal": "take over the world 🌍🌎🌏"
  }
}
`

const sprintUpdatedEndDate = `{
  "timestamp": 1596144049198,
  "webhookEvent": "sprint_updated",
  "sprint": {
    "id": 196,
    "self": "https://pinpt-hq.atlassian.net/rest/agile/1.0/sprint/196",
    "state": "active",
    "name": "TES Sprint 2",
    "startDate": "2020-07-30T21:13:24.588Z",
    "endDate": "2020-08-14T21:13:00.000Z",
    "originBoardId": 9,
    "goal": "take over the world! 🌍🌎🌏"
  },
  "oldValue": {
    "id": 196,
    "self": "https://pinpt-hq.atlassian.net/rest/agile/1.0/sprint/196",
    "state": "active",
    "name": "TES Sprint 2",
    "startDate": "2020-07-30T21:13:24.588Z",
    "endDate": "2020-08-13T21:13:00.000Z",
    "originBoardId": 9,
    "goal": "take over the world! 🌍🌎🌏"
  }
}`

func TestWebhookBuildSprintUpdateChangeName(t *testing.T) {
	assert := assert.New(t)
	val, changed := buildSprintUpdate(sprintProjection{
		ID:   1,
		Name: "Sprint 1",
	}, sprintProjection{
		ID:   1,
		Name: "Sprint! 1",
	})
	assert.True(changed)
	assert.NotNil(val.Set.Name)
	assert.EqualValues("Sprint! 1", *val.Set.Name)
}

func TestWebhookBuildSprintUpdateChangeGoal(t *testing.T) {
	assert := assert.New(t)
	val, changed := buildSprintUpdate(sprintProjection{
		ID:   1,
		Name: "Sprint 1",
		Goal: sdk.StringPointer("hello"),
	}, sprintProjection{
		ID:   1,
		Name: "Sprint 1",
		Goal: sdk.StringPointer("hello world!"),
	})
	assert.True(changed)
	assert.Nil(val.Set.Name)
	assert.NotNil(val.Set.Goal)
	assert.EqualValues("hello world!", *val.Set.Goal)
}

func TestWebhookJiraSprintUpdateStarted(t *testing.T) {
	assert := assert.New(t)
	pipe := &sdktest.MockPipe{}
	i := JiraIntegration{
		logger: sdk.NewNoOpTestLogger(),
	}
	assert.NoError(i.webhookUpdateSprint("1234", "1", loadFile("testdata/sprint_updated.json"), pipe))
	assert.Len(pipe.Written, 1)
	update := pipe.Written[0].(*agent.UpdateData)
	assert.EqualValues("", update.Set["active"])
	assert.EqualValues("\"ACTIVE\"", update.Set["status"])
	assert.EqualValues("{\"epoch\":1596143604588,\"offset\":0,\"rfc3339\":\"2020-07-30T21:13:24.588+00:00\"}", update.Set["started_date"])
	assert.EqualValues("{\"epoch\":1597353180000,\"offset\":0,\"rfc3339\":\"2020-08-13T21:13:00+00:00\"}", update.Set["ended_date"])
}

func TestWebhookJiraSprintUpdateGoalSet(t *testing.T) {
	assert := assert.New(t)
	pipe := &sdktest.MockPipe{}
	i := JiraIntegration{
		logger: sdk.NewNoOpTestLogger(),
	}
	assert.NoError(i.webhookUpdateSprint("1234", "1", []byte(sprintUpdateGoalAdded), pipe))
	assert.Len(pipe.Written, 1)
	update := pipe.Written[0].(*agent.UpdateData)
	assert.EqualValues("", update.Set["status"])
	assert.EqualValues("", update.Set["name"])
	assert.EqualValues("\"take over the world 🌍🌎🌏\"", update.Set["goal"])
}

func TestWebhookJiraSprintUpdateGoalUpdated(t *testing.T) {
	assert := assert.New(t)
	pipe := &sdktest.MockPipe{}
	i := JiraIntegration{
		logger: sdk.NewNoOpTestLogger(),
	}
	assert.NoError(i.webhookUpdateSprint("1234", "1", []byte(sprintUpdateGoalUpdated), pipe))
	assert.Len(pipe.Written, 1)
	update := pipe.Written[0].(*agent.UpdateData)
	assert.EqualValues("", update.Set["status"])
	assert.EqualValues("", update.Set["name"])
	assert.EqualValues("", update.Set["ended_date"])
	assert.EqualValues("", update.Set["started_date"])
	assert.EqualValues("\"take over the world! 🌍🌎🌏\"", update.Set["goal"])
}

func TestWebhookJiraSprintUpdateEndDate(t *testing.T) {
	assert := assert.New(t)
	pipe := &sdktest.MockPipe{}
	i := JiraIntegration{
		logger: sdk.NewNoOpTestLogger(),
	}
	assert.NoError(i.webhookUpdateSprint("1234", "1", []byte(sprintUpdatedEndDate), pipe))
	assert.Len(pipe.Written, 1)
	update := pipe.Written[0].(*agent.UpdateData)
	assert.EqualValues("", update.Set["status"])
	assert.EqualValues("", update.Set["name"])
	assert.EqualValues("", update.Set["goal"])
	assert.EqualValues("{\"epoch\":1597439580000,\"offset\":0,\"rfc3339\":\"2020-08-14T21:13:00+00:00\"}", update.Set["ended_date"])
	assert.EqualValues("", update.Set["started_date"])
}

const sprintUpdateNothing = `{
  "timestamp": 1596142226397,
  "webhookEvent": "sprint_updated",
  "sprint": {
    "id": 196,
    "self": "https://pinpt-hq.atlassian.net/rest/agile/1.0/sprint/196",
    "state": "future",
    "name": "TES Sprint 2",
    "originBoardId": 9
  },
  "oldValue": {
    "id": 196,
    "self": "https://pinpt-hq.atlassian.net/rest/agile/1.0/sprint/196",
    "state": "future",
    "name": "TES Sprint 2",
    "originBoardId": 9
  }
}`

func TestWebhookJiraSprintUpdateNothing(t *testing.T) {
	assert := assert.New(t)
	pipe := &sdktest.MockPipe{}
	i := JiraIntegration{
		logger: sdk.NewNoOpTestLogger(),
	}
	assert.NoError(i.webhookUpdateSprint("1234", "1", []byte(sprintUpdateNothing), pipe))
	assert.Len(pipe.Written, 0)
}
