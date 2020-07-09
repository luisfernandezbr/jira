package internal

import (
	"fmt"

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

type issueTypesResult struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (i *JiraIntegration) fetchIssueTypesForProject(state *state, projectRefID string) ([]sdk.WorkProjectIssueTypes, error) {
	theurl := sdk.JoinURL(state.authConfig.APIURL, "rest/api/3/project/"+projectRefID+"/statuses")
	client := i.httpmanager.New(theurl, nil)
	results := make([]sdk.WorkProjectIssueTypes, 0)
	resp := make([]issueTypesResult, 0)
	if _, err := client.Get(&resp, state.authConfig.Middleware...); err != nil {
		return nil, fmt.Errorf("error fetching issue types: %w", err)
	}
	for _, r := range resp {
		results = append(results, sdk.WorkProjectIssueTypes{
			Name:  r.Name,
			RefID: r.ID,
		})
	}
	return results, nil
}
