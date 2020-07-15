package internal

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/pinpt/adf"
	"github.com/pinpt/agent.next/sdk"
)

// easyjson:skip
type customFieldIDs struct {
	StoryPoints string
	Epic        string
	StartDate   string
	EndDate     string
}

// easyjson:skip
type customFieldValue struct {
	ID    string
	Name  string
	Value string
}

// easyjson:skip
type customField struct {
	ID   string
	Name string
}

func (s customFieldIDs) missing() (res []string) {
	if s.StoryPoints == "" {
		res = append(res, "StoryPoints")
	}
	if s.Epic == "" {
		res = append(res, "Epic")
	}
	return
}

// ToModel will convert a issueSource (from Jira) to a sdk.WorkIssue object
func (i issueSource) ToModel(customerID string, integrationInstanceID string, issueManager *issueIDManager, sprintManager *sprintManager, userManager *userManager, fieldByID map[string]customField, websiteURL string) (*sdk.WorkIssue, []*sdk.WorkIssueComment, error) {
	var fields issueFields
	if err := sdk.MapToStruct(i.Fields, &fields); err != nil {
		return nil, nil, err
	}

	// map of issue keys that this issue is dependent on
	transitiveIssueKeys := make(map[string]bool)

	issue := &sdk.WorkIssue{}
	issue.Active = true
	issue.CustomerID = customerID
	issue.RefID = i.ID
	issue.RefType = refType
	issue.Identifier = i.Key
	issue.ProjectID = sdk.NewWorkProjectID(customerID, fields.Project.ID, refType)
	issue.ID = sdk.NewWorkIssueID(customerID, i.ID, refType)
	issue.IntegrationInstanceID = sdk.StringPointer(integrationInstanceID)

	customFields := make([]customFieldValue, 0)

	if fields.DueDate != "" {
		orig := fields.DueDate
		d, err := time.ParseInLocation("2006-01-02", orig, time.UTC)
		if err != nil {
			return nil, nil, fmt.Errorf("could not parse duedate of jira issue: %v err: %v", i.Key, err)
		}
		sdk.ConvertTimeToDateModel(d, &issue.DueDate)
	}

	issue.Title = fields.Summary

	if fields.Description != nil {
		html, err := adf.GenerateHTMLFromADF(fields.Description)
		if err != nil {
			return nil, nil, fmt.Errorf("could not parse description for jira issue: %v err: %v", i.Key, err)
		}
		issue.Description = adjustRenderedHTML(websiteURL, html)
	}

	comments := make([]*sdk.WorkIssueComment, 0)

	for _, comment := range fields.Comment.Comments {
		thecomment, err := comment.ToModel(customerID, integrationInstanceID, websiteURL, userManager, issue.ProjectID, issue.ID, i.Key)
		if err != nil {
			return nil, nil, fmt.Errorf("could create issue comment for jira issue: %v err: %v", i.Key, err)
		}
		comments = append(comments, thecomment)
	}

	created, err := parseTime(fields.Created)
	if err != nil {
		return nil, nil, err
	}
	sdk.ConvertTimeToDateModel(created, &issue.CreatedDate)
	updated, err := parseTime(fields.Updated)
	if err != nil {
		return nil, nil, err
	}
	sdk.ConvertTimeToDateModel(updated, &issue.UpdatedDate)

	issue.Priority = fields.Priority.Name
	issue.PriorityID = sdk.NewWorkIssuePriorityID(customerID, refType, fields.Priority.ID)
	issue.Type = fields.IssueType.Name
	issue.TypeID = sdk.NewWorkIssueTypeID(customerID, refType, fields.IssueType.ID)
	issue.Status = fields.Status.Name
	issue.Resolution = fields.Resolution.Name
	if fields.Parent != nil && fields.Parent.ID != "" {
		issue.ParentID = sdk.NewWorkIssueID(customerID, fields.Parent.ID, refType)
	}

	if !fields.Creator.IsZero() {
		issue.CreatorRefID = fields.Creator.RefID()
		if err := userManager.emit(fields.Creator); err != nil {
			return nil, nil, err
		}
	}
	if !fields.Reporter.IsZero() {
		issue.ReporterRefID = fields.Reporter.RefID()
		if err := userManager.emit(fields.Reporter); err != nil {
			return nil, nil, err
		}
	}
	if !fields.Assignee.IsZero() {
		issue.AssigneeRefID = fields.Assignee.RefID()
		if err := userManager.emit(fields.Assignee); err != nil {
			return nil, nil, err
		}
	}

	issue.URL = issueURL(websiteURL, i.Key)
	issue.Tags = fields.Labels

	for _, link := range fields.IssueLinks {
		var linkType sdk.WorkIssueLinkedIssuesLinkType
		reverseDirection := false
		switch link.Type.Name {
		case "Blocks":
			linkType = sdk.WorkIssueLinkedIssuesLinkTypeBlocks
		case "Cloners":
			linkType = sdk.WorkIssueLinkedIssuesLinkTypeClones
		case "Duplicate":
			linkType = sdk.WorkIssueLinkedIssuesLinkTypeDuplicates
		case "Problem/Incident":
			linkType = sdk.WorkIssueLinkedIssuesLinkTypeCauses
		case "Relates":
			linkType = sdk.WorkIssueLinkedIssuesLinkTypeRelates
		default:
			// we only support default names
			continue
		}
		var linkedIssue linkedIssue
		if link.OutwardIssue.ID != "" {
			linkedIssue = link.OutwardIssue
		} else if link.InwardIssue.ID != "" {
			linkedIssue = link.InwardIssue
			reverseDirection = true
		} else {
			continue
		}
		link2 := sdk.WorkIssueLinkedIssues{}
		link2.RefID = link.ID
		link2.IssueID = sdk.NewWorkIssueID(customerID, linkedIssue.ID, refType)
		link2.IssueRefID = linkedIssue.ID
		link2.IssueIdentifier = linkedIssue.Key
		link2.ReverseDirection = reverseDirection
		link2.LinkType = linkType
		issue.LinkedIssues = append(issue.LinkedIssues, link2)
		transitiveIssueKeys[linkedIssue.Key] = true
	}

	for _, data := range fields.Attachment {
		var attachment sdk.WorkIssueAttachments
		attachment.RefID = data.ID
		attachment.Name = data.Filename
		attachment.URL = data.Content
		attachment.ThumbnailURL = data.Thumbnail
		attachment.MimeType = data.MimeType
		attachment.Size = int64(data.Size)
		user := data.Author.AccountID // cloud
		if user == "" {
			user = data.Author.Key // hosted
		}
		attachment.UserRefID = user
		created, err := parseTime(data.Created)
		if err != nil {
			return nil, nil, err
		}
		sdk.ConvertTimeToDateModel(created, &attachment.CreatedDate)
		issue.Attachments = append(issue.Attachments, attachment)
	}

	for k, v := range i.Fields {
		if strings.HasPrefix(k, "customfield_") && v != nil {
			if arr, ok := v.([]interface{}); ok && len(arr) != 0 {
				for _, each := range arr {
					str, ok := each.(string)
					if !ok {
						continue
					}
					id := extractPossibleSprintID(str)
					if id == "" {
						continue
					}
					issue.SprintIds = append(issue.SprintIds, sdk.NewAgileSprintID(customerID, id, refType))
				}
			}
		}
	}

	customFieldIDs := customFieldIDs{}

	for key, val := range fieldByID {
		switch val.Name {
		case "Story Points":
			customFieldIDs.StoryPoints = key
		case "Epic Link":
			customFieldIDs.Epic = key
		case "Start Date":
			customFieldIDs.StartDate = key
		case "End Date":
			customFieldIDs.EndDate = key
		}
	}

	var epicKey string

	for k, d := range i.Fields {
		if !strings.HasPrefix(k, "customfield_") {
			continue
		}
		fd, ok := fieldByID[k]
		if !ok {
			continue
		}
		v := ""
		if d != nil {
			ds, ok := d.(string)
			if ok {
				v = ds
			} else {
				b, err := json.Marshal(d)
				if err != nil {
					return nil, nil, err
				}
				v = string(b)
			}
		}

		f := customFieldValue{}
		f.ID = fd.ID
		f.Name = fd.Name
		f.Value = v
		customFields = append(customFields, f)

		if v == "" {
			continue
		}
		switch fd.ID {
		case customFieldIDs.StartDate:
			d, err := parsePlannedDate(v)
			if err != nil {
				continue
			}
			sdk.ConvertTimeToDateModel(d, &issue.PlannedStartDate)
		case customFieldIDs.EndDate:
			d, err := parsePlannedDate(v)
			if err != nil {
				continue
			}
			sdk.ConvertTimeToDateModel(d, &issue.PlannedEndDate)
		case customFieldIDs.StoryPoints:
			// story points can be expressed as fractions or whole numbers so convert it to a float
			sp, err := strconv.ParseFloat(v, 32)
			if err == nil {
				issue.StoryPoints = &sp
			}
		case customFieldIDs.Epic:
			transitiveIssueKeys[v] = true
			epicKey = v // will get set below
		}
	}

	issueRefID := issue.RefID

	// ordinal should be a monotonically increasing number for changelogs
	// the value itself doesn't matter as long as the changelog is from
	// the oldest to the newest
	//
	// Using current timestamp here instead of int, so the number is also an increasing one when running incrementals compared to previous values in the historical.
	ordinal := sdk.EpochNow()

	// Jira changelog histories are ordered from the newest to the oldest but we want changelogs to be
	// sent from the oldest event to the newest event when we send
	for h := len(i.Changelog.Histories) - 1; h >= 0; h-- {
		cl := i.Changelog.Histories[h]
		for _, data := range cl.Items {

			item := sdk.WorkIssueChangeLog{}
			item.RefID = cl.ID
			item.Ordinal = ordinal

			ordinal++

			createdAt, err := parseTime(cl.Created)
			if err != nil {
				return nil, nil, fmt.Errorf("could not parse created time of changelog for issue: %v err: %v", issueRefID, err)
			}
			sdk.ConvertTimeToDateModel(createdAt, &issue.CreatedDate)
			item.UserID = cl.Author.RefID()

			item.FromString = data.FromString + " @ " + data.From
			item.ToString = data.ToString + " @ " + data.To

			switch strings.ToLower(data.Field) {
			case "status":
				item.Field = sdk.WorkIssueChangeLogFieldStatus
				item.From = data.FromString
				item.To = data.ToString
			case "resolution":
				item.Field = sdk.WorkIssueChangeLogFieldResolution
				item.From = data.FromString
				item.To = data.ToString
			case "assignee":
				item.Field = sdk.WorkIssueChangeLogFieldAssigneeRefID
				if data.From != "" {
					item.From = data.From
				}
				if data.To != "" {
					item.To = data.To
				}
			case "reporter":
				item.Field = sdk.WorkIssueChangeLogFieldReporterRefID
				item.From = data.From
				item.To = data.To
			case "summary":
				item.Field = sdk.WorkIssueChangeLogFieldTitle
				item.From = data.FromString
				item.To = data.ToString
			case "duedate":
				item.Field = sdk.WorkIssueChangeLogFieldDueDate
				item.From = data.FromString
				item.To = data.ToString
			case "issuetype":
				item.Field = sdk.WorkIssueChangeLogFieldType
				item.From = data.FromString
				item.To = data.ToString
			case "labels":
				item.Field = sdk.WorkIssueChangeLogFieldTags
				item.From = data.FromString
				item.To = data.ToString
			case "priority":
				item.Field = sdk.WorkIssueChangeLogFieldPriority
				item.From = data.FromString
				item.To = data.ToString
			case "project":
				item.Field = sdk.WorkIssueChangeLogFieldProjectID
				if data.From != "" {
					item.From = sdk.NewWorkProjectID(customerID, data.From, refType)
				}
				if data.To != "" {
					item.From = sdk.NewWorkProjectID(customerID, data.To, refType)
				}
			case "key":
				item.Field = sdk.WorkIssueChangeLogFieldIdentifier
				item.From = data.FromString
				item.To = data.ToString
			case "sprint":
				item.Field = sdk.WorkIssueChangeLogFieldSprintIds
				var from, to []string
				if data.From != "" {
					for _, s := range strings.Split(data.From, ",") {
						from = append(from, sdk.NewAgileSprintID(customerID, strings.TrimSpace(s), refType))
					}
				}
				if data.To != "" {
					for _, s := range strings.Split(data.To, ",") {
						to = append(to, sdk.NewAgileSprintID(customerID, strings.TrimSpace(s), refType))
					}
				}
				item.From = strings.Join(from, ",")
				item.To = strings.Join(to, ",")
			case "parent":
				item.Field = sdk.WorkIssueChangeLogFieldParentID
				if data.From != "" {
					item.From = sdk.NewWorkIssueID(customerID, data.From, refType)
					transitiveIssueKeys[data.From] = true
				}
				if data.To != "" {
					item.To = sdk.NewWorkIssueID(customerID, data.To, refType)
					transitiveIssueKeys[data.To] = true
				}
			case "epic link":
				item.Field = sdk.WorkIssueChangeLogFieldEpicID
				if data.From != "" {
					item.From = sdk.NewWorkIssueID(customerID, data.From, refType)
					transitiveIssueKeys[data.From] = true
				}
				if data.To != "" {
					item.To = sdk.NewWorkIssueID(customerID, data.To, refType)
					transitiveIssueKeys[data.To] = true
				}
			default:
				// Ignore other change types
				continue
			}
			issue.ChangeLog = append(issue.ChangeLog, item)
		}
	}

	// handle transition mapping
	issue.Transitions = make([]sdk.WorkIssueTransitions, 0)
	for _, t := range i.Transitions {
		// transition will include the current status which is a bit weird so exclude that
		if t.Name != issue.Status {
			issue.Transitions = append(issue.Transitions, sdk.WorkIssueTransitions{
				Name:  t.Name,
				RefID: t.ID, // the transition id, not the issue type id
			})
		}
	}

	// now go in one shot and resolve all transitive issue keys
	if len(transitiveIssueKeys) > 0 {
		keys := sdk.Keys(transitiveIssueKeys)
		found, err := issueManager.getRefIDsFromKeys(keys)
		if err != nil {
			return nil, nil, err
		}
		// if we have an epic key target, find it and then set it on our issue
		if epicKey != "" {
			for pos, key := range keys {
				if key == epicKey {
					refID := found[pos]
					epicID := sdk.NewWorkIssueID(customerID, refID, refType)
					issue.EpicID = &epicID
					break
				}
			}
		}
	}

	if !sprintManager.usingAgileAPI {
		// process any sprint information on this issue
		for _, field := range customFields {
			if field.Name == "Sprint" {
				if field.Value == "" {
					continue
				}
				data, err := parseSprints(field.Value)
				if err != nil {
					return nil, nil, err
				}
				for _, s := range data {
					if err := sprintManager.emit(s); err != nil {
						return nil, nil, err
					}
				}
				break
			}
		}
	}

	return issue, comments, nil
}

// easyjson:skip
type issueIDManager struct {
	refids        map[string]string
	logger        sdk.Logger
	i             *JiraIntegration
	export        sdk.Export
	pipe          sdk.Pipe
	fields        map[string]customField
	sprintManager *sprintManager
	userManager   *userManager
	authConfig    authConfig
	stats         *stats
}

func newIssueIDManager(logger sdk.Logger, i *JiraIntegration, export sdk.Export, pipe sdk.Pipe, sprintManager *sprintManager, userManager *userManager, fields map[string]customField, authConfig authConfig, stats *stats) *issueIDManager {
	return &issueIDManager{
		refids:        make(map[string]string),
		i:             i,
		logger:        logger,
		authConfig:    authConfig,
		sprintManager: sprintManager,
		userManager:   userManager,
		export:        export,
		pipe:          pipe,
		fields:        fields,
		stats:         stats,
	}
}

func (m *issueIDManager) cache(key string, refid string) {
	m.refids[key] = refid
}

func (m *issueIDManager) isProcessed(key string) bool {
	return m.refids[key] != ""
}

func (m *issueIDManager) getRefIDsFromKeys(keys []string) ([]string, error) {
	found := make([]string, 0)
	foundkeys := make(map[string]bool)
	notfound := make([]string, 0)
	for _, key := range keys {
		refid := m.refids[key]
		if refid != "" {
			found = append(found, refid)
			foundkeys[key] = true
			foundkeys[refid] = true
		} else {
			notfound = append(notfound, key)
		}
	}
	// since we can have both KEY and REFID in the list we need to go back through and
	// remove any already found
	for i, f := range notfound {
		if foundkeys[f] {
			if len(notfound) > i {
				notfound = append(notfound[:i], notfound[i+1:]...)
			} else {
				notfound = notfound[:i]
			}
		}
	}
	// if we found all the keys requested, just return
	if len(found) == len(keys) {
		return found, nil
	}
	// we have to go to Jira and fetch the keys we don't have locally
	theurl := sdk.JoinURL(m.authConfig.APIURL, "/rest/api/3/search")
	sdk.LogDebug(m.logger, "fetching dependent issues", "notfound", notfound, "found", found)
	qs := url.Values{}
	qs.Set("jql", "key IN ("+strings.Join(notfound, ",")+")")
	qs.Set("expand", "changelog,fields,comments,transitions")
	qs.Set("fields", "*navigable,attachment")
	var result issueQueryResult
	client := m.i.httpmanager.New(theurl, nil)
	for {
		resp, err := client.Get(&result, append(m.authConfig.Middleware, sdk.WithGetQueryParameters(qs))...)
		if resp == nil && err != nil {
			return nil, err
		}
		for _, issue := range result.Issues {
			// cache it before so we don't get into recursive loops
			m.refids[issue.Key] = issue.ID
			m.refids[issue.ID] = issue.ID
		}
		for _, issue := range result.Issues {
			// recursively process it
			issueObject, comments, err := issue.ToModel(m.export.CustomerID(), m.export.IntegrationInstanceID(), m, m.sprintManager, m.userManager, m.fields, m.authConfig.WebsiteURL)
			if err != nil {
				return nil, err
			}
			if err := m.pipe.Write(issueObject); err != nil {
				return nil, err
			}
			for _, comment := range comments {
				if err := m.pipe.Write(comment); err != nil {
					return nil, err
				}
				m.stats.incComment()
			}
			m.stats.incIssue()
			if rerr := m.i.checkForRateLimit(m.export, m.export.CustomerID(), err, resp.Headers); rerr != nil {
				return nil, rerr
			}
		}
		res := make([]string, 0)
		for _, key := range keys {
			res = append(res, m.refids[key])
		}
		// return in the order in which they came in
		return res, nil
	}
}

const epicCustomFieldIDCacheKey = "epic_id_custom_field"

func (i *JiraIntegration) updateIssue(state *state, mutation sdk.Mutation, event *sdk.WorkIssueUpdateMutation) error {
	started := time.Now()
	var hasMutation bool
	updateMutation := newMutation()
	if event.Set.Title != nil {
		updateMutation.Update["summary"] = []setMutationOperation{
			setMutationOperation{
				Set: event.Set.Title,
			},
		}
		hasMutation = true
	}
	if event.Set.Priority != nil {
		updateMutation.Update["priority"] = []setMutationOperation{
			setMutationOperation{
				Set: idValue{*event.Set.Priority.ID},
			},
		}
		hasMutation = true
	}
	if event.Set.Epic != nil || event.Unset.Epic {
		var epicFieldID string
		if ok, _ := mutation.State().Get(epicCustomFieldIDCacheKey, &epicFieldID); !ok {
			// fetch the custom fields and find the custom field value for the Epic Link
			customfields, err := i.fetchCustomFields(state.logger, mutation, mutation.CustomerID(), state.authConfig)
			if err != nil {
				return fmt.Errorf("error fetching custom fields for setting the epic id. %w", err)
			}
			for _, field := range customfields {
				if field.Name == "Epic Link" {
					epicFieldID = field.ID
					mutation.State().Set(epicCustomFieldIDCacheKey, epicFieldID)
					break
				}
			}
		}
		if event.Unset.Epic {
			updateMutation.Update[epicFieldID] = []setMutationOperation{
				setMutationOperation{
					Set: nil,
				},
			}
		} else {
			updateMutation.Update[epicFieldID] = []setMutationOperation{
				setMutationOperation{
					Set: *event.Set.Epic.Name, // we use the name which should be set to the identifier in the case of an epic
				},
			}
		}
		hasMutation = true
	}
	sdk.LogDebug(state.logger, "sending mutation", "payload", sdk.Stringify(updateMutation), "has_mutation", hasMutation)
	if hasMutation {
		theurl := sdk.JoinURL(state.authConfig.APIURL, "/rest/api/3/issue/"+mutation.ID())
		client := i.httpmanager.New(theurl, nil)
		_, err := client.Put(sdk.StringifyReader(updateMutation), nil, state.authConfig.Middleware...)
		if err != nil {
			return fmt.Errorf("mutation failed: %s", getJiraErrorMessage(err))
		}
	}
	if event.Set.Transition != nil {
		updateMutation = newMutation()
		updateMutation.Transition = &idValue{*event.Set.Transition.ID}
		if event.Set.Resolution != nil {
			if event.Set.Resolution.Name == nil {
				return fmt.Errorf("resolution name property must be set")
			}
			updateMutation.Fields = map[string]interface{}{
				"resolution": map[string]string{"name": *event.Set.Resolution.Name},
			}
		}
		sdk.LogDebug(state.logger, "sending transition mutation", "payload", sdk.Stringify(updateMutation))
		theurl := sdk.JoinURL(state.authConfig.APIURL, "/rest/api/3/issue/"+mutation.ID()+"/transitions")
		client := i.httpmanager.New(theurl, nil)
		_, err := client.Post(sdk.StringifyReader(updateMutation), nil, state.authConfig.Middleware...)
		if err != nil {
			return fmt.Errorf("mutation transition failed: %s", getJiraErrorMessage(err))
		}
	}
	sdk.LogDebug(state.logger, "completed mutation response", "payload", sdk.Stringify(updateMutation), "duration", time.Since(started))
	return nil
}
