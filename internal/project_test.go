package internal

import (
	"encoding/json"
	"sort"
	"testing"

	"github.com/pinpt/agent/v4/sdk"
	"github.com/stretchr/testify/assert"
)

func TestSortableFields(t *testing.T) {
	assert := assert.New(t)
	vals := mutationFieldsSortable{
		{
			RefID: "components",
		},
		{
			RefID: "assignee",
		},
		{
			RefID: "summary",
		},
		{
			RefID: "description",
		},
	}
	sort.Sort(vals)
	assert.Equal("summary", vals[0].RefID)
	assert.Equal("description", vals[1].RefID)
	assert.Equal("assignee", vals[2].RefID)
	assert.Equal("components", vals[3].RefID)
}

func TestSortableFieldsWithCustom(t *testing.T) {
	assert := assert.New(t)
	vals := mutationFieldsSortable{
		{
			RefID: "fooo",
		},
		{
			RefID: "bar",
		},
		{
			RefID: "components",
		},
		{
			RefID: "assignee",
		},
		{
			RefID: "summary",
		},
		{
			RefID: "description",
		},
	}
	sort.Sort(vals)
	assert.Equal("summary", vals[0].RefID)
	assert.Equal("description", vals[1].RefID)
	assert.Equal("assignee", vals[2].RefID)
	assert.Equal("components", vals[3].RefID)
	assert.True(vals[4].RefID == "fooo" || vals[4].RefID == "bar")
	assert.True(vals[5].RefID == "fooo" || vals[5].RefID == "bar")
}

func TestCreateThing(t *testing.T) {
	assert := assert.New(t)
	buf := loadFile("testdata/issue_createmeta.json")
	var resp issueCreateMeta
	assert.NoError(json.Unmarshal(buf, &resp))
	fields, err := createMutationFields(resp.Projects[0])
	assert.NoError(err)
	assert.Len(fields, 6)
	var i int
	assert.Equal("issuetype", fields[i].RefID)
	assert.Equal(sdk.WorkProjectCapabilityIssueMutationFieldsTypeWorkIssueType, fields[i].Type)
	assert.Len(fields[i].RequiredByTypes, 7)
	assert.True(fields[i].AlwaysRequired)
	i++
	assert.Equal("summary", fields[i].RefID)
	assert.Equal(sdk.WorkProjectCapabilityIssueMutationFieldsTypeString, fields[i].Type)
	assert.Len(fields[i].RequiredByTypes, 7)
	assert.True(fields[i].AlwaysRequired)
	i++
	assert.Equal("description", fields[i].RefID)
	assert.Equal(sdk.WorkProjectCapabilityIssueMutationFieldsTypeString, fields[i].Type)
	assert.Len(fields[i].RequiredByTypes, 0)
	assert.False(fields[i].AlwaysRequired)
	i++
	assert.Equal("priority", fields[i].RefID)
	assert.Equal(sdk.WorkProjectCapabilityIssueMutationFieldsTypeWorkIssuePriority, fields[i].Type)
	assert.Len(fields[i].RequiredByTypes, 0)
	assert.False(fields[i].AlwaysRequired)
	i++
	assert.Equal("assignee", fields[i].RefID)
	assert.Equal(sdk.WorkProjectCapabilityIssueMutationFieldsTypeUser, fields[i].Type)
	assert.Len(fields[i].RequiredByTypes, 0)
	assert.False(fields[i].AlwaysRequired)
	i++
	assert.Equal("customfield_10003", fields[i].RefID)
	assert.Equal("Epic Name", fields[i].Name)
	assert.Equal(sdk.WorkProjectCapabilityIssueMutationFieldsTypeString, fields[i].Type)
	assert.Len(fields[i].RequiredByTypes, 1)
	assert.Equal([]string{"10000"}, fields[i].RequiredByTypes)
	assert.False(fields[i].AlwaysRequired)

}
