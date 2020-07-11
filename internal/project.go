package internal

import (
	"time"

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

const projectCapabilityStateKeyPrefix = "project_capability_"

func (i *JiraIntegration) createProjectCapability(state sdk.State, jiraProject project, project *sdk.WorkProject, historical bool) (*sdk.WorkProjectCapability, error) {
	key := projectCapabilityStateKeyPrefix + project.ID
	if !historical && state.Exists(key) {
		return nil, nil
	}
	var capability sdk.WorkProjectCapability
	capability.CustomerID = project.CustomerID
	capability.RefID = project.RefID
	capability.RefType = project.RefType
	capability.ProjectID = project.ID
	capability.IntegrationInstanceID = project.IntegrationInstanceID
	capability.Attachments = true
	capability.ChangeLogs = true
	capability.DueDates = true
	capability.Epics = true
	capability.InProgressStates = true
	// TODO: would be nice to figure out if this project uses Kanban, Scrum or both
	capability.KanbanBoards = true
	capability.LinkedIssues = true
	capability.Parents = true
	if jiraProject.Simplified && jiraProject.Style == "next-gen" {
		capability.Priorities = false // next gen project doesn't have priorities
	} else {
		capability.Priorities = true
	}
	capability.Resolutions = true
	capability.Sprints = true
	capability.StoryPoints = true
	if err := state.SetWithExpires(key, 1, time.Hour*24*30); err != nil {
		return nil, err
	}
	return &capability, nil
}
