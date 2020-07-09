package internal

import (
	"github.com/pinpt/agent.next/sdk"
)

func (p project) ToModel(customerID string, integrationInstanceID string, websiteURL string, issueTypes []sdk.WorkProjectIssueTypes, resolutions []sdk.WorkProjectIssueResolutions) (*sdk.WorkProject, error) {
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
	project.Affiliation = sdk.WorkProjectAffiliationOrganization
	project.Visibility = sdk.WorkProjectVisibilityPrivate
	project.Name = p.Name
	project.URL = projectURL(websiteURL, p.Key)
	project.IssueTypes = issueTypes
	project.IssueResolutions = resolutions
	return project, nil
}
