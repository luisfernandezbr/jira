package internal

import (
	"encoding/json"
	"errors"
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
	projectCapabilityStateKeyPrefixLegacy = "project_capability2_"
	projectCapabilityStateKeyPrefix       = "project_capability3_"
)

// its a float so that we can insert stuff betwen ints
var builtInFieldOrder = map[string]float32{
	"issuetype":   0,
	"summary":     1,
	"description": 2,
	"priority":    3,
	"assignee":    4,
	"parent":      5,
	"components":  6,
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
	switch field.Key {
	case "issuetype":
		return sdk.WorkProjectCapabilityIssueMutationFields{
			Description: sdk.StringPointer("The type of issue."),
			Name:        "Issue Type",
			RefID:       "issuetype",
			Type:        sdk.WorkProjectCapabilityIssueMutationFieldsTypeWorkIssueType,
		}, true, nil
	case "summary":
		return sdk.WorkProjectCapabilityIssueMutationFields{
			Description: sdk.StringPointer("The title for this issue"),
			Name:        "Summary",
			RefID:       "summary",
			Type:        sdk.WorkProjectCapabilityIssueMutationFieldsTypeString,
		}, true, nil
	case "description":
		return sdk.WorkProjectCapabilityIssueMutationFields{
			Description: sdk.StringPointer("The description of the issue."),
			Name:        "Description",
			RefID:       "description",
			Type:        sdk.WorkProjectCapabilityIssueMutationFieldsTypeTextbox,
		}, true, nil
	case "priority":
		return sdk.WorkProjectCapabilityIssueMutationFields{
			Description: sdk.StringPointer("The priority for the issue."),
			Name:        "Priority",
			RefID:       "priority",
			Type:        sdk.WorkProjectCapabilityIssueMutationFieldsTypeWorkIssuePriority,
		}, true, nil
	case "assignee":
		return sdk.WorkProjectCapabilityIssueMutationFields{
			Description: sdk.StringPointer("The assignee for the issue."),
			Name:        "Assignee",
			RefID:       "assignee",
			Type:        sdk.WorkProjectCapabilityIssueMutationFieldsTypeUser,
		}, true, nil
	case "parent":
		return sdk.WorkProjectCapabilityIssueMutationFields{
			Description: sdk.StringPointer("The parent of the issue, should not be another Sub-Task."),
			Name:        "Parent",
			RefID:       "parent",
			Type:        sdk.WorkProjectCapabilityIssueMutationFieldsTypeWorkIssue,
		}, true, nil
	case "components":
		if string(field.AllowedValues) == "[]" {
			break
		}
		var vals []sdk.WorkProjectCapabilityIssueMutationFieldsValues
		var components []allowedValueComponent
		if err := json.Unmarshal(field.AllowedValues, &components); err != nil {
			return sdk.WorkProjectCapabilityIssueMutationFields{}, false, fmt.Errorf("error decoding components: %w", err)
		}
		for _, component := range components {
			vals = append(vals, sdk.WorkProjectCapabilityIssueMutationFieldsValues{
				RefID: &component.RefID,
				Name:  &component.Name,
			})
		}
		if len(vals) == 0 {
			break
		}
		return sdk.WorkProjectCapabilityIssueMutationFields{
			Description: sdk.StringPointer("Components for the issue."),
			Name:        "Components",
			RefID:       "components",
			Type:        sdk.WorkProjectCapabilityIssueMutationFieldsTypeStringArray,
			Values:      vals,
		}, true, nil
	default:
		// try matching by name
		switch field.Name {
		case "Epic Link":
			return sdk.WorkProjectCapabilityIssueMutationFields{
				Description: sdk.StringPointer("The epic this issue is part of"),
				Name:        field.Name,
				RefID:       field.Key,
				Type:        sdk.WorkProjectCapabilityIssueMutationFieldsTypeEpic,
			}, true, nil
		case "Epic Name":
			return sdk.WorkProjectCapabilityIssueMutationFields{
				Description: sdk.StringPointer("The short name for this epic"),
				Name:        field.Name,
				RefID:       field.Key,
				Type:        sdk.WorkProjectCapabilityIssueMutationFieldsTypeString,
			}, true, nil
		}
	}
	return sdk.WorkProjectCapabilityIssueMutationFields{}, false, nil
}

func convertSchemaType(schemaType string) (sdk.WorkProjectCapabilityIssueMutationFieldsType, error) {
	switch schemaType {
	case "string":
		return sdk.WorkProjectCapabilityIssueMutationFieldsTypeString, nil
	case "number":
		return sdk.WorkProjectCapabilityIssueMutationFieldsTypeNumber, nil
	case "issuelink":
		return sdk.WorkProjectCapabilityIssueMutationFieldsTypeWorkIssue, nil
	}
	return 0, errUnsupportedField
}

var excludedFields = map[string]bool{
	"project":  true,
	"reporter": true, // using a user's tokens implies a reporter
}

var errUnsupportedField = errors.New("field was unsupported")

func createMutationFields(createMeta projectIssueCreateMeta) ([]sdk.WorkProjectCapabilityIssueMutationFields, error) {
	existingFields := make(map[string]*sdk.WorkProjectCapabilityIssueMutationFields)
	for _, issueType := range createMeta.Issuetypes {
		typeRefID := issueType.ID
		for _, field := range issueType.Fields {
			fieldRefID := field.Key
			if excludedFields[fieldRefID] {
				continue
			}
			existing := existingFields[fieldRefID]
			if existing != nil {
				// if we have already found this field append this type to it
				if field.Required {
					existing.RequiredByTypes = append(existing.RequiredByTypes, typeRefID)
				}
				existing.AvailableForTypes = append(existing.AvailableForTypes, typeRefID)
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
				newField.AvailableForTypes = []string{typeRefID}
				if isBuiltInField {
					existingFields[fieldRefID] = &newField
				} else {
					// new non-builtin field
					if field.Required {
						fieldType, err := convertSchemaType(field.Schema.Type)
						if err != nil {
							return nil, fmt.Errorf("error converting required field %s of type %s: %w", field.Name, field.Schema.Type, err)
						}
						existingFields[fieldRefID] = &sdk.WorkProjectCapabilityIssueMutationFields{
							RequiredByTypes:   []string{typeRefID},
							AvailableForTypes: []string{typeRefID},
							Name:              field.Name,
							RefID:             field.Key,
							Type:              fieldType,
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
		if field == nil {
			continue
		}
		if len(field.RequiredByTypes) == issueCount {
			field.AlwaysRequired = true
		}
		if len(field.AvailableForTypes) == issueCount {
			field.AlwaysAvailable = true
		}
		mutFields = append(mutFields, *field)
	}
	sort.Sort(mutationFieldsSortable(mutFields))
	return mutFields, nil
}

func (i *JiraIntegration) createProjectCapability(logger sdk.Logger, state sdk.State, jiraProject project, project *sdk.WorkProject, getCreateMeta func() (*projectIssueCreateMeta, error), historical bool) (*sdk.WorkProjectCapability, error) {
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
	if createMeta != nil {
		// NOTE: sometimes projects don't have this, need to investigate further
		capability.IssueMutationFields, err = createMutationFields(*createMeta)
		if err != nil {
			if errors.Is(err, errUnsupportedField) {
				sdk.LogWarn(logger, "project had an unsupported field, disabling issue mutations", "err", err)
			} else {
				return nil, err
			}
		}
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
