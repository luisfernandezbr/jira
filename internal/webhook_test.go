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
