package internal

import "github.com/pinpt/agent.next/sdk"

func (p project) ToModel(customerID string, integrationInstanceID string) (*sdk.WorkProject, error) {
	project := &sdk.WorkProject{}
	project.CustomerID = customerID
	project.IntegrationInstanceID = sdk.StringPointer(integrationInstanceID)
	project.RefID = p.ID
	project.RefType = refType
	project.Description = sdk.StringPointer(p.Description)
	project.Category = sdk.StringPointer(p.ProjectCategory.Name)
	project.Active = true
	project.Identifier = p.Key
	project.ID = sdk.NewWorkProjectID(customerID, p.ID, refType)
	return project, nil
}
