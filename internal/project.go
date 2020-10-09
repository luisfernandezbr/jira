package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"time"

	"github.com/pinpt/agent/v4/sdk"
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

const (
	projectCapabilityStateKeyPrefixLegacy = "project_capability_"
	projectCapabilityStateKeyPrefix       = "project_capability2_"
)

// its a float so that we can insert stuff betwen ints
var builtInFieldOrder = map[string]float32{
	"issuetype":   0,
	"summary":     1,
	"description": 2,
	"priority":    3,
	"assignee":    4,
	"components":  5,
}

type mutationFieldsSortable []sdk.WorkProjectCapabilityIssueMutationFields

var _ sort.Interface = (*mutationFieldsSortable)(nil)

func (m mutationFieldsSortable) Len() int      { return len(m) }
func (m mutationFieldsSortable) Swap(i, j int) { m[i], m[j] = m[j], m[i] }
func (m mutationFieldsSortable) Less(i, j int) bool {
	iVal, iisbuiltin := builtInFieldOrder[m[i].RefID]
	jVal, jisbuiltin := builtInFieldOrder[m[j].RefID]
	if iisbuiltin && jisbuiltin {
		return iVal < jVal
	}
	return iisbuiltin && !jisbuiltin
}

func handleBuiltinField(field issueTypeField) (sdk.WorkProjectCapabilityIssueMutationFields, bool, error) {
	var newField sdk.WorkProjectCapabilityIssueMutationFields
	switch field.Key {
	case "issuetype":
		newField = sdk.WorkProjectCapabilityIssueMutationFields{
			Description: sdk.StringPointer("The type of issue."),
			Name:        "Issue Type",
			RefID:       "issuetype",
			Type:        sdk.WorkProjectCapabilityIssueMutationFieldsTypeWorkIssueType,
		}
	case "summary":
		newField = sdk.WorkProjectCapabilityIssueMutationFields{
			Description: sdk.StringPointer("The title for this issue"),
			Name:        "Summary",
			RefID:       "summary",
			Type:        sdk.WorkProjectCapabilityIssueMutationFieldsTypeString,
		}
	case "description":
		newField = sdk.WorkProjectCapabilityIssueMutationFields{
			Description: sdk.StringPointer("The description of the issue."),
			Name:        "Description",
			RefID:       "description",
			Type:        sdk.WorkProjectCapabilityIssueMutationFieldsTypeString,
		}
	case "priority":
		newField = sdk.WorkProjectCapabilityIssueMutationFields{
			Description: sdk.StringPointer("The priority for the issue."),
			Name:        "Priority",
			RefID:       "priority",
			Type:        sdk.WorkProjectCapabilityIssueMutationFieldsTypeWorkIssuePriority,
		}
	case "assignee":
		newField = sdk.WorkProjectCapabilityIssueMutationFields{
			Description: sdk.StringPointer("The assignee for the issue."),
			Name:        "Assignee",
			RefID:       "assignee",
			Type:        sdk.WorkProjectCapabilityIssueMutationFieldsTypeUser,
		}
	case "components":
		newField = sdk.WorkProjectCapabilityIssueMutationFields{
			Description: sdk.StringPointer("Components for the issue."),
			Name:        "Components",
			RefID:       "components",
			Type:        sdk.WorkProjectCapabilityIssueMutationFieldsTypeStringArray,
		}
		if string(field.AllowedValues) != "[]" {
			var components []allowedValueComponent
			if err := json.Unmarshal(field.AllowedValues, &components); err != nil {
				return sdk.WorkProjectCapabilityIssueMutationFields{}, false, fmt.Errorf("error decoding components: %w", err)
			}
			for _, component := range components {
				newField.Values = append(newField.Values, sdk.WorkProjectCapabilityIssueMutationFieldsValues{
					RefID: &component.RefID,
					Name:  &component.Name,
				})
			}
			break
		}
		fallthrough
	default:
		return sdk.WorkProjectCapabilityIssueMutationFields{}, false, nil
	}

	return newField, true, nil
}

func convertSchemaType(schemaType string) (sdk.WorkProjectCapabilityIssueMutationFieldsType, bool) {
	switch schemaType {
	case "string":
		return sdk.WorkProjectCapabilityIssueMutationFieldsTypeString, true
	case "number":
		return sdk.WorkProjectCapabilityIssueMutationFieldsTypeNumber, true
	}
	return 0, false
}

var excludedFields = map[string]bool{
	"project":  true,
	"reporter": true, // using a user's tokens implies a reporter
}

func createMutationFields(createMeta projectIssueCreateMeta) ([]sdk.WorkProjectCapabilityIssueMutationFields, error) {
	existingFields := make(map[string]*sdk.WorkProjectCapabilityIssueMutationFields)
	for _, issueType := range createMeta.Issuetypes {
		typeRefID := issueType.ID
		for _, field := range issueType.Fields {
			refID := field.Key
			if excludedFields[refID] {
				continue
			}
			existing := existingFields[refID]
			if existing != nil {
				// if we have already found this field append this type to it
				if field.Required {
					existing.RequiredByTypes = append(existing.RequiredByTypes, typeRefID)
				}
			} else {
				// first time encountering this field, check if its a builtin
				newField, isBuiltInField, err := handleBuiltinField(field)
				if err != nil {
					return nil, err
				}
				if field.Required {
					newField.RequiredByTypes = []string{typeRefID}
				} else {
					newField.RequiredByTypes = make([]string, 0)
				}
				if isBuiltInField {
					existingFields[newField.RefID] = &newField
				} else {
					// new non-builtin field
					if field.Required {
						fieldType, ok := convertSchemaType(field.Schema.Type)
						if ok {
							existingFields[newField.RefID] = &sdk.WorkProjectCapabilityIssueMutationFields{
								RequiredByTypes: []string{typeRefID},
								Name:            field.Name,
								RefID:           field.Key,
								Type:            fieldType,
							}
						}
					}
					// ignore non required fields
				}
			}
		}
	}
	issueCount := len(createMeta.Issuetypes)
	var mutFields []sdk.WorkProjectCapabilityIssueMutationFields
	for _, field := range existingFields {
		if len(field.RequiredByTypes) == issueCount {
			field.AlwaysRequired = true
		}
		if field == nil {
			continue
		}
		mutFields = append(mutFields, *field)
	}
	sort.Sort(mutationFieldsSortable(mutFields))
	return mutFields, nil
}

func (i *JiraIntegration) createProjectCapability(state sdk.State, jiraProject project, project *sdk.WorkProject, getCreateMeta func() (projectIssueCreateMeta, error), historical bool) (*sdk.WorkProjectCapability, error) {
	key := projectCapabilityStateKeyPrefix + project.ID
	// Delete old project capability state
	state.Delete(projectCapabilityStateKeyPrefixLegacy + project.ID)
	if !historical && state.Exists(key) {
		return nil, nil
	}
	createMeta, err := getCreateMeta()
	if err != nil {
		return nil, err
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
	capability.IssueMutationFields, err = createMutationFields(createMeta)
	if err != nil {
		return nil, err
	}
	if err := state.SetWithExpires(key, 1, time.Hour*24*30); err != nil {
		return nil, err
	}
	return &capability, nil
}

func setProjectExpand(qs url.Values) {
	qs.Set("expand", "description,url,issueTypes,projectKeys,insight")
}

func (i *JiraIntegration) fetchProject(state *state, customerID, refID string) (*sdk.WorkProject, error) {
	theurl := sdk.JoinURL(state.authConfig.APIURL, "/rest/api/3/project/", refID)
	client := i.httpmanager.New(theurl, nil)
	qs := url.Values{}
	setProjectExpand(qs)
	var p project
	resp, err := client.Get(&p, append(state.authConfig.Middleware, sdk.WithGetQueryParameters(qs))...)
	if resp == nil && err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	resolutions, err := i.fetchIssueResolutions(state)
	if err != nil {
		return nil, err
	}
	issueTypes, err := i.fetchIssueTypesForProject(state, p.ID)
	if err != nil {
		return nil, err
	}
	return p.ToModel(customerID, state.integrationInstanceID, state.authConfig.WebsiteURL, issueTypes, resolutions)
}
