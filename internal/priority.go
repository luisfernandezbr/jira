package internal

import (
	"github.com/pinpt/agent.next/sdk"
)

func (p issuePriority) ToModel(customerID string, integrationInstanceID string) (*sdk.WorkIssuePriority, error) {
	priority := &sdk.WorkIssuePriority{}
	priority.CustomerID = customerID
	priority.IntegrationInstanceID = sdk.StringPointer(integrationInstanceID)
	priority.RefID = p.ID
	priority.RefType = refType
	priority.Name = p.Name
	priority.Description = sdk.StringPointer(p.Description)
	priority.IconURL = sdk.StringPointer(p.IconURL)
	priority.Color = sdk.StringPointer(p.StatusColor)
	priority.ID = sdk.NewWorkIssuePriorityID(customerID, refType, p.ID)
	return priority, nil
}
