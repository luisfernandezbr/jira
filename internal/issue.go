package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/pinpt/adf"
	"github.com/pinpt/agent/sdk"
)

// easyjson:skip
type customFieldIDs struct {
	StoryPoints string
	Epic        string
	StartDate   string
	EndDate     string
	Sprint      string
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

func extractSprints(fields map[string]interface{}, ids customFieldIDs) ([]sprint, bool, error) {
	if ids.Sprint != "" {
		if blob, ok := fields[ids.Sprint]; ok {
			buf, err := json.Marshal(blob)
			if err != nil {
				return nil, false, fmt.Errorf("error reencoding sprint custom field: %w", err)
			}
			var sprints []sprint
			if err := json.Unmarshal(buf, &sprints); err != nil {
				return nil, false, fmt.Errorf("error decoding sprint custom field: %w", err)
			}
			return sprints, true, nil
		}
	}
	return nil, false, nil
}

func createChangeLog(customerID string, refID string, userRefID string, createdAt time.Time, ordinal int64, item changeLogItem) *sdk.WorkIssueChangeLog {
	change := sdk.WorkIssueChangeLog{
		RefID:   refID,
		UserID:  userRefID,
		Ordinal: ordinal,
	}

	sdk.ConvertTimeToDateModel(createdAt, &change.CreatedDate)

	// TODO(robin): remove this once everything is working
	change.FromString = item.FromString + " @ " + item.From
	change.ToString = item.ToString + " @ " + item.To

	switch strings.ToLower(item.Field) {
	case "status":
		change.Field = sdk.WorkIssueChangeLogFieldStatus
		change.From = item.FromString
		change.To = item.ToString
	case "resolution":
		change.Field = sdk.WorkIssueChangeLogFieldResolution
		change.From = item.FromString
		change.To = item.ToString
	case "assignee":
		change.Field = sdk.WorkIssueChangeLogFieldAssigneeRefID
		if item.From != "" {
			change.From = item.From
		}
		if item.To != "" {
			change.To = item.To
		}
	case "reporter":
		change.Field = sdk.WorkIssueChangeLogFieldReporterRefID
		change.From = item.From
		change.To = item.To
	case "summary":
		change.Field = sdk.WorkIssueChangeLogFieldTitle
		change.From = item.FromString
		change.To = item.ToString
	case "duedate":
		change.Field = sdk.WorkIssueChangeLogFieldDueDate
		change.From = item.From
		change.To = item.To
	case "issuetype":
		change.Field = sdk.WorkIssueChangeLogFieldType
		change.From = item.FromString
		change.To = item.ToString
	case "labels":
		change.Field = sdk.WorkIssueChangeLogFieldTags
		change.From = item.FromString
		change.To = item.ToString
	case "priority":
		change.Field = sdk.WorkIssueChangeLogFieldPriority
		change.From = item.FromString
		change.To = item.ToString
	case "project":
		change.Field = sdk.WorkIssueChangeLogFieldProjectID
		if item.From != "" {
			change.From = sdk.NewWorkProjectID(customerID, item.From, refType)
		}
		if item.To != "" {
			change.To = sdk.NewWorkProjectID(customerID, item.To, refType)
		}
	case "key":
		change.Field = sdk.WorkIssueChangeLogFieldIdentifier
		change.From = item.FromString
		change.To = item.ToString
	case "sprint":
		change.Field = sdk.WorkIssueChangeLogFieldSprintIds
		var from, to []string
		if item.From != "" {
			for _, s := range strings.Split(item.From, ",") {
				from = append(from, sdk.NewAgileSprintID(customerID, strings.TrimSpace(s), refType))
			}
		}
		if item.To != "" {
			for _, s := range strings.Split(item.To, ",") {
				to = append(to, sdk.NewAgileSprintID(customerID, strings.TrimSpace(s), refType))
			}
		}
		change.From = strings.Join(from, ",")
		change.To = strings.Join(to, ",")
	case "parent":
		change.Field = sdk.WorkIssueChangeLogFieldParentID
		if item.From != "" {
			change.From = sdk.NewWorkIssueID(customerID, item.From, refType)
		}
		if item.To != "" {
			change.To = sdk.NewWorkIssueID(customerID, item.To, refType)
		}
	case "epic link":
		change.Field = sdk.WorkIssueChangeLogFieldEpicID
		if item.From != "" {
			change.From = sdk.NewWorkIssueID(customerID, item.From, refType)
		}
		if item.To != "" {
			change.To = sdk.NewWorkIssueID(customerID, item.To, refType)
		}
	default:
		return nil
	}
	return &change
}

func makeTransitions(currentStatus string, raw []transitionSource) []sdk.WorkIssueTransitions {
	transitions := make([]sdk.WorkIssueTransitions, 0)
	for _, t := range raw {
		// transition will include the current status which is a bit weird so exclude that
		if t.Name != currentStatus {
			tx := sdk.WorkIssueTransitions{
				Name:  t.Name,
				RefID: t.ID, // the transition id, not the issue type id
			}
			if t.To.StatusCategory.Key == statusCategoryDone {
				tx.Terminal = true
				tx.Requires = []string{sdk.WorkIssueTransitionRequiresResolution}
			}
			transitions = append(transitions, tx)
		}
	}
	return transitions
}

// ToModel will convert a issueSource (from Jira) to a sdk.WorkIssue object
func (i issueSource) ToModel(customerID string, integrationInstanceID string, issueManager *issueIDManager, sprintManager *sprintManager, userManager UserManager, fieldByID map[string]customField, websiteURL string, fetchTransitive bool) (*sdk.WorkIssue, []*sdk.WorkIssueComment, error) {
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
	issue.StatusID = sdk.NewWorkIssueStatusID(customerID, refType, fields.Status.ID)
	issue.Resolution = fields.Resolution.Name

	if fields.Parent != nil && fields.Parent.ID != "" {
		issue.ParentID = sdk.NewWorkIssueID(customerID, fields.Parent.ID, refType)
	}

	if !fields.Creator.IsZero() {
		issue.CreatorRefID = fields.Creator.RefID()
		if err := userManager.Emit(fields.Creator); err != nil {
			return nil, nil, err
		}
	}
	if !fields.Reporter.IsZero() {
		issue.ReporterRefID = fields.Reporter.RefID()
		if err := userManager.Emit(fields.Reporter); err != nil {
			return nil, nil, err
		}
	}
	if !fields.Assignee.IsZero() {
		issue.AssigneeRefID = fields.Assignee.RefID()
		if err := userManager.Emit(fields.Assignee); err != nil {
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

	customFieldIDs := customFieldIDs{}

	for key, val := range fieldByID {
		switch val.Name {
		case "Story Points":
			customFieldIDs.StoryPoints = key
		case "Sprint":
			customFieldIDs.Sprint = key
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
		var v string
		if d != nil {
			if ds, ok := d.(string); ok {
				v = ds
			} else {
				b, err := json.Marshal(d)
				if err != nil {
					return nil, nil, err
				}
				v = string(b)
			}
		}
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

	sprints, foundSprintIDs, err := extractSprints(i.Fields, customFieldIDs)
	if err != nil {
		return nil, nil, err
	}

	if foundSprintIDs {
		for _, sprint := range sprints {
			issue.SprintIds = append(issue.SprintIds, sdk.NewAgileSprintID(customerID, strconv.Itoa(sprint.ID), refType))
		}
	} else {
		// try to extract sprint_ids the old way
		// TODO(robin): check that this does anything for jira on premise
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
	}

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
			ordinal++
			createdAt, err := parseTime(cl.Created)
			if err != nil {
				return nil, nil, fmt.Errorf("could not parse created time of changelog for issue: %v err: %v", issue.RefID, err)
			}
			item := createChangeLog(customerID, cl.ID, cl.Author.RefID(), createdAt, ordinal, data)
			if item == nil {
				continue
			}
			if item.Field == sdk.WorkIssueChangeLogFieldParentID || item.Field == sdk.WorkIssueChangeLogFieldEpicID {
				transitiveIssueKeys[data.To] = true
				transitiveIssueKeys[data.From] = true
			}
			issue.ChangeLog = append(issue.ChangeLog, *item)
		}
	}

	// handle transition mapping
	issue.Transitions = makeTransitions(issue.Status, i.Transitions)

	// now go in one shot and resolve all transitive issue keys
	if len(transitiveIssueKeys) > 0 && fetchTransitive {
		delete(transitiveIssueKeys, "")
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
		if foundSprintIDs {
			for _, s := range sprints {
				if err := sprintManager.emit(s); err != nil {
					return nil, nil, err
				}
			}
		} else {
			// process any sprint information on this issue
			if b, ok := i.Fields[customFieldIDs.Sprint]; ok && b != nil {
				buf, err := json.Marshal(b)
				if err != nil {
					return nil, nil, fmt.Errorf("error marshalling sprint field: %w", err)
				}
				data, err := parseSprints(string(buf))
				if err != nil {
					return nil, nil, err
				}
				for _, s := range data {
					if err := sprintManager.emit(s); err != nil {
						return nil, nil, err
					}
				}
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
	control       sdk.Control
	pipe          sdk.Pipe
	fields        map[string]customField
	sprintManager *sprintManager
	userManager   UserManager
	authConfig    authConfig
	stats         *stats
}

func newIssueIDManager(logger sdk.Logger, i *JiraIntegration, control sdk.Control, pipe sdk.Pipe, sprintManager *sprintManager, userManager UserManager, fields map[string]customField, authConfig authConfig, stats *stats) *issueIDManager {
	return &issueIDManager{
		refids:        make(map[string]string),
		i:             i,
		logger:        logger,
		authConfig:    authConfig,
		sprintManager: sprintManager,
		userManager:   userManager,
		control:       control,
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
	setIssueExpand(qs)
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
			issueObject, comments, err := issue.ToModel(m.control.CustomerID(), m.control.IntegrationInstanceID(), m, m.sprintManager, m.userManager, m.fields, m.authConfig.WebsiteURL, true)
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
			if rerr := m.i.checkForRateLimit(m.control, m.control.CustomerID(), err, resp.Headers); rerr != nil {
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

// fetch just one issue by refid, and optionally fetch any other transitive/mentioned issues
func (m *issueIDManager) fetchIssue(refid string, fetchTransitive bool) (*sdk.WorkIssue, []*sdk.WorkIssueComment, error) {
	theurl := sdk.JoinURL(m.authConfig.APIURL, "/rest/api/3/issue/", refid)
	client := m.i.httpmanager.New(theurl, nil)
	qs := url.Values{}
	setIssueExpand(qs)
	qs.Set("fields", "*navigable,attachment")
	var issue issueSource
	resp, err := client.Get(&issue, append(m.authConfig.Middleware, sdk.WithGetQueryParameters(qs))...)
	if resp == nil && err != nil {
		return nil, nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil, nil
	}
	return issue.ToModel(m.control.CustomerID(), m.control.IntegrationInstanceID(), m, m.sprintManager, m.userManager, m.fields, m.authConfig.WebsiteURL, fetchTransitive)
}

func setIssueExpand(qs url.Values) {
	qs.Set("expand", "changelog,fields,comments,transitions")
}

const epicCustomFieldIDCacheKey = "epic_id_custom_field"

func (i *JiraIntegration) updateIssue(logger sdk.Logger, mutation sdk.Mutation, authConfig authConfig, event *sdk.WorkIssueUpdateMutation) error {
	started := time.Now()
	var hasMutation bool
	updateMutation := newMutation()
	if event.Set.Title != nil {
		updateMutation.Update["summary"] = []setMutationOperation{
			{
				Set: event.Set.Title,
			},
		}
		hasMutation = true
	}
	if event.Set.Priority != nil {
		updateMutation.Update["priority"] = []setMutationOperation{
			{
				Set: idValue{*event.Set.Priority.RefID},
			},
		}
		hasMutation = true
	}
	if event.Set.AssigneeRefID != nil {
		assigneeRefID := *event.Set.AssigneeRefID
		if assigneeRefID == "" {
			// null value means unassigned
			updateMutation.Update["assignee"] = []setMutationOperation{
				{
					Set: userValue{AccountID: nil},
				},
			}
		} else {
			updateMutation.Update["assignee"] = []setMutationOperation{
				{
					Set: userValue{AccountID: &assigneeRefID},
				},
			}
		}
		hasMutation = true
	}
	if event.Set.Epic != nil || event.Unset.Epic {
		var epicFieldID string
		if ok, _ := mutation.State().Get(epicCustomFieldIDCacheKey, &epicFieldID); !ok {
			// fetch the custom fields and find the custom field value for the Epic Link
			customfields, err := i.fetchCustomFields(logger, mutation, mutation.CustomerID(), authConfig)
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
				{
					Set: nil,
				},
			}
		} else {
			updateMutation.Update[epicFieldID] = []setMutationOperation{
				{
					Set: *event.Set.Epic.Name, // we use the name which should be set to the identifier in the case of an epic
				},
			}
		}
		hasMutation = true
	}
	sdk.LogDebug(logger, "sending mutation", "payload", sdk.Stringify(updateMutation), "has_mutation", hasMutation)
	if hasMutation {
		theurl := sdk.JoinURL(authConfig.APIURL, "/rest/api/3/issue/"+mutation.ID())
		client := i.httpmanager.New(theurl, nil)
		if _, err := client.Put(sdk.StringifyReader(updateMutation), nil, authConfig.Middleware...); err != nil {
			return fmt.Errorf("mutation failed: %s", getJiraErrorMessage(err))
		}
	}
	if event.Set.Transition != nil {
		if event.Set.Transition.RefID == nil {
			return fmt.Errorf("error ref_id was nil for transition: %v", event.Set.Transition)
		}
		updateMutation = newMutation()
		updateMutation.Transition = &idValue{*event.Set.Transition.RefID}
		if event.Set.Resolution != nil {
			if event.Set.Resolution.Name == nil {
				return fmt.Errorf("resolution name property must be set")
			}
			updateMutation.Fields = map[string]interface{}{
				"resolution": map[string]string{"name": *event.Set.Resolution.Name},
			}
		}
		sdk.LogDebug(logger, "sending transition mutation", "payload", sdk.Stringify(updateMutation))
		theurl := sdk.JoinURL(authConfig.APIURL, "/rest/api/3/issue/"+mutation.ID()+"/transitions")
		client := i.httpmanager.New(theurl, nil)
		_, err := client.Post(sdk.StringifyReader(updateMutation), nil, authConfig.Middleware...)
		if err != nil {
			return fmt.Errorf("mutation transition failed: %s", getJiraErrorMessage(err))
		}
	}
	sdk.LogDebug(logger, "completed mutation response", "payload", sdk.Stringify(updateMutation), "duration", time.Since(started))
	return nil
}
