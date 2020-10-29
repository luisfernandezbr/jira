package internal

import (
	"encoding/json"
	"errors"
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
	assert.Len(fields, 8)
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
	assert.Len(fields[i].AvailableForTypes, 7)
	assert.False(fields[i].AlwaysRequired)
	i++
	assert.Equal("priority", fields[i].RefID)
	assert.Equal(sdk.WorkProjectCapabilityIssueMutationFieldsTypeWorkIssuePriority, fields[i].Type)
	assert.Len(fields[i].RequiredByTypes, 0)
	assert.Len(fields[i].AvailableForTypes, 7)
	assert.False(fields[i].AlwaysRequired)
	i++
	assert.Equal("assignee", fields[i].RefID)
	assert.Equal(sdk.WorkProjectCapabilityIssueMutationFieldsTypeUser, fields[i].Type)
	assert.Len(fields[i].RequiredByTypes, 0)
	assert.Len(fields[i].AvailableForTypes, 7)
	assert.False(fields[i].AlwaysRequired)
	i++
	assert.Equal("parent", fields[i].RefID)
	assert.Equal("Parent", fields[i].Name)
	assert.Equal(sdk.WorkProjectCapabilityIssueMutationFieldsTypeWorkIssue, fields[i].Type)
	assert.Len(fields[i].RequiredByTypes, 1)
	assert.Equal([]string{"10102"}, fields[i].RequiredByTypes)
	assert.Len(fields[i].AvailableForTypes, 1)
	assert.Equal("10102", fields[i].AvailableForTypes[0])
	assert.False(fields[i].AlwaysRequired)
	i++
	assert.True("Epic Name" == fields[i].Name || "Epic Link" == fields[i].Name)
	if "Epic Name" == fields[i].Name {
		s := mutationFieldsSortable(fields)
		s.Swap(i, i+1)
	}
	assert.Equal("customfield_10006", fields[i].RefID)
	assert.Equal("Epic Link", fields[i].Name)
	assert.Equal(sdk.WorkProjectCapabilityIssueMutationFieldsTypeEpic, fields[i].Type)
	assert.Len(fields[i].RequiredByTypes, 0)
	assert.Len(fields[i].AvailableForTypes, 7)
	assert.False(fields[i].AlwaysRequired)
	i++
	assert.Equal("customfield_10003", fields[i].RefID)
	assert.Equal("Epic Name", fields[i].Name)
	assert.Equal(sdk.WorkProjectCapabilityIssueMutationFieldsTypeString, fields[i].Type)
	assert.Len(fields[i].RequiredByTypes, 1)
	assert.Equal([]string{"10000"}, fields[i].RequiredByTypes)
	assert.Len(fields[i].AvailableForTypes, 1)
	assert.Equal("10000", fields[i].AvailableForTypes[0])
	assert.False(fields[i].AlwaysRequired)

}

func TestCantMakeMutationFields(t *testing.T) {
	assert := assert.New(t)
	buf := loadFile("testdata/issue_createmeta.json")
	var resp issueCreateMeta
	assert.NoError(json.Unmarshal(buf, &resp))
	p := projectIssueCreateMeta{
		Issuetypes: []createMetaIssueTypes{
			{
				ID: "Task",
				Fields: map[string]issueTypeField{
					"summary": {
						Required:        true,
						Schema:          issueTypeFieldSchema{Type: "string", System: "summary"},
						Name:            "Summary",
						Key:             "summary",
						HasDefaultValue: false,
					},
					"customfield_10520": {
						Required:        true,
						Schema:          issueTypeFieldSchema{Type: "any", System: "wahtever"},
						Name:            "Enviornment",
						Key:             "env",
						HasDefaultValue: false,
					},
				},
			},
		},
	}
	fields, err := createMutationFields(p)
	assert.EqualError(err, "error converting required field Enviornment of type any: field was unsupported")
	assert.True(errors.Is(err, errUnsupportedField))
	assert.Empty(fields)
}
