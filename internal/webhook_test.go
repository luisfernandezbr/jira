package internal

import (
	"io/ioutil"
	"testing"

	"github.com/pinpt/agent.next/sdk"
	"github.com/pinpt/agent.next/sdk/sdktest"
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
}
