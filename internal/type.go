package internal

import (
	"github.com/pinpt/agent.next/sdk"
)

func (t issueType) ToModel(customerID string) (*sdk.WorkIssueType, error) {
	issuetype := &sdk.WorkIssueType{}
	issuetype.CustomerID = customerID
	issuetype.RefID = t.ID
	issuetype.RefType = refType
	issuetype.Name = t.Name
	issuetype.Description = sdk.StringPointer(t.Description)
	issuetype.IconURL = sdk.StringPointer(t.Icon)
	issuetype.ID = sdk.NewWorkIssueTypeID(customerID, refType, t.ID)
	return issuetype, nil
}
