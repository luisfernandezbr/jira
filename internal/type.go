package internal

import (
	"github.com/pinpt/agent.next/sdk"
)

func getMappedIssueType(name string, subtask bool) sdk.WorkIssueTypeMappedType {
	if subtask {
		// any subtask will have this flag set
		return sdk.WorkIssueTypeMappedTypeSubtask
	}
	// map out of the box jira types that are known
	switch name {
	case "Story":
		return sdk.WorkIssueTypeMappedTypeStory
	case "Improvement", "Enhancement":
		return sdk.WorkIssueTypeMappedTypeEnhancement
	case "Epic":
		return sdk.WorkIssueTypeMappedTypeEpic
	case "New Feature":
		return sdk.WorkIssueTypeMappedTypeFeature
	case "Bug":
		return sdk.WorkIssueTypeMappedTypeBug
	case "Task":
		return sdk.WorkIssueTypeMappedTypeTask
	}
	// otherwise this is a custom type which can be mapped from the app side by the user
	return sdk.WorkIssueTypeMappedTypeUnknown
}

func (t issueType) ToModel(customerID string) (*sdk.WorkIssueType, error) {
	issuetype := &sdk.WorkIssueType{}
	issuetype.CustomerID = customerID
	issuetype.RefID = t.ID
	issuetype.RefType = refType
	issuetype.Name = t.Name
	issuetype.Description = sdk.StringPointer(t.Description)
	issuetype.IconURL = sdk.StringPointer(t.Icon)
	issuetype.MappedType = getMappedIssueType(t.Name, t.Subtask)
	issuetype.ID = sdk.NewWorkIssueTypeID(customerID, refType, t.ID)
	return issuetype, nil
}
