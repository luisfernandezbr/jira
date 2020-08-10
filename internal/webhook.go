package internal

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pinpt/agent.next/sdk"
)

var webhookEvents = []string{
	"jira:issue_created",
	"jira:issue_updated",
	"jira:issue_deleted",
	"comment_created",
	"comment_updated",
	"comment_deleted",
	"attachment_created",
	"attachment_deleted",
	"issuelink_created",
	"issuelink_deleted",
	"project_created",
	"project_updated",
	"project_deleted",
	"user_created",
	"user_updated",
	"user_deleted",
	"sprint_created",
	"sprint_deleted",
	"sprint_updated",
	"sprint_started",
	"sprint_closed",
	"board_created",
	"board_updated",
	"board_deleted",
	"board_configuration_changed",
}

const webhookVersion = "1" // change this to have the webhook uninstalled and reinstalled new

func (i *JiraIntegration) uninstallWebHookIfNecessary(logger sdk.Logger, config sdk.Config, state sdk.State, authConfig authConfig, customerID string, integrationInstanceID string) error {
	if config.BasicAuth == nil {
		sdk.LogInfo(logger, "skipping web hook uninstall since not using basic auth")
	}
	// fetch the webhooks for this instance and delete them
	client := i.httpmanager.New(sdk.JoinURL(config.BasicAuth.URL, "/webhooks/1.0/webhook"), nil)
	var resp []struct {
		Name string `json:"name"`
		Self string `json:"self"`
	}
	if _, err := client.Get(&resp, authConfig.Middleware...); err == nil {
		for _, r := range resp {
			if r.Name == "Pinpoint/"+integrationInstanceID {
				c := i.httpmanager.New(r.Self, nil)
				var res interface{}
				c.Delete(&res, authConfig.Middleware...)
				sdk.LogDebug(logger, "removed jira webhook at "+r.Self)
			}
		}
	}
	if err := i.manager.WebHookManager().Delete(customerID, integrationInstanceID, refType, "", sdk.WebHookScopeOrg); err != nil {
		return err
	}
	sdk.LogInfo(logger, "org web hook removed")
	return nil
}

func (i *JiraIntegration) installWebHookIfNecessary(logger sdk.Logger, config sdk.Config, state sdk.State, authConfig authConfig, customerID string, integrationInstanceID string) error {
	if config.BasicAuth == nil {
		sdk.LogInfo(logger, "skipping web hook install since not using basic auth")
		return nil
	}
	if i.manager.WebHookManager().Exists(customerID, integrationInstanceID, refType, "", sdk.WebHookScopeOrg) {
		url, err := i.manager.WebHookManager().HookURL(customerID, integrationInstanceID, refType, "", sdk.WebHookScopeOrg)
		if err != nil {
			return err
		}
		// check and see if we need to upgrade our webhook
		if strings.Contains(url, "&version="+webhookVersion) {
			sdk.LogInfo(logger, "skipping web hook install since already installed")
			return nil
		}
		// we have a webhook, but it's an older version so let's remove and re-add
		i.uninstallWebHookIfNecessary(logger, config, state, authConfig, customerID, integrationInstanceID)
	}
	webhookurl, err := i.manager.WebHookManager().Create(customerID, integrationInstanceID, refType, "", sdk.WebHookScopeOrg, "version="+webhookVersion)
	if err != nil {
		return fmt.Errorf("error creating webhook url: %w", err)
	}
	url := sdk.JoinURL(config.BasicAuth.URL, "/webhooks/1.0/webhook")
	client := i.httpmanager.New(url, nil)
	req := map[string]interface{}{
		"name":   "Pinpoint/" + integrationInstanceID,
		"url":    webhookurl,
		"events": webhookEvents,
	}
	var res struct {
		Self string `json:"self"`
	}
	if _, err := client.Post(sdk.StringifyReader(req), &res, authConfig.Middleware...); err != nil {
		// mark the webhook as errored
		sdk.LogInfo(logger, "error installing org webhook", "err", err)
		i.manager.WebHookManager().Errored(customerID, integrationInstanceID, refType, "", sdk.WebHookScopeOrg, err)
		return nil
	}
	sdk.LogInfo(logger, "installed webhook", "id", res.Self)
	return nil
}

type webhookEvent struct {
	Event string `json:"webhookEvent"`
}

func (i *JiraIntegration) webhookUpdateIssue(webhook sdk.WebHook) error {
	rawdata := webhook.Bytes()
	customerID := webhook.CustomerID()
	integrationInstanceID := webhook.IntegrationInstanceID()
	pipe := webhook.Pipe()
	var changelog struct {
		Timestamp int64 `json:"timestamp"`
		User      user  `json:"user"`
		Issue     struct {
			ID     string `json:"id"`
			Key    string `json:"key"`
			Fields struct {
				Project struct {
					ID string `json:"id"`
				} `json:"project"`
			} `json:"fields"`
		}
		Changelog struct {
			ID    string `json:"id"`
			Items []struct {
				Field      string `json:"field"`
				FieldType  string `json:"fieldtype"`
				From       string `json:"from"`
				FromString string `json:"fromString"`
				To         string `json:"to"`
				ToString   string `json:"toString"`
			} `json:"items"`
		} `json:"changelog"`
	}
	if err := json.Unmarshal(rawdata, &changelog); err != nil {
		return fmt.Errorf("error parsing json for changelog: %w", err)
	}
	ts := sdk.DateFromEpoch(changelog.Timestamp)
	val := sdk.WorkIssueUpdate{}
	var updatedStatus bool
	for i, change := range changelog.Changelog.Items {
		var field sdk.WorkIssueChangeLogField
		var skip bool
		switch change.Field {
		case "summary":
			field = sdk.WorkIssueChangeLogFieldTitle
			val.Set.Title = sdk.StringPointer(change.ToString)
		case "description":
			val.Set.Description = sdk.StringPointer(change.ToString)
			skip = true // TODO: add description to the datamodel so we can send it in changelog
		case "status":
			field = sdk.WorkIssueChangeLogFieldStatus
			val.Set.Status = &sdk.NameID{
				Name: sdk.StringPointer(change.ToString),
				ID:   sdk.StringPointer(sdk.NewWorkIssueStatusID(customerID, refType, change.To)),
			}
			updatedStatus = true
		case "Epic Link":
			field = sdk.WorkIssueChangeLogFieldEpicID
			if change.To == "" {
				val.Unset.EpicID = sdk.BoolPointer(true)
			} else {
				val.Set.EpicID = sdk.StringPointer(sdk.NewWorkIssueID(customerID, change.To, refType))
			}
		case "priority":
			field = sdk.WorkIssueChangeLogFieldPriority
			val.Set.Priority = &sdk.NameID{
				Name: sdk.StringPointer(change.ToString),
				ID:   sdk.StringPointer(sdk.NewWorkIssuePriorityID(customerID, refType, change.To)),
			}
		case "assignee":
			field = sdk.WorkIssueChangeLogFieldAssigneeRefID
			val.Set.AssigneeRefID = sdk.StringPointer(change.To)
		case "labels":
			field = sdk.WorkIssueChangeLogFieldTags
			tags := strings.Split(change.ToString, " ")
			val.Set.Tags = &tags
			change.To = change.ToString // to is null, this api is lousy
		case "resolution":
			field = sdk.WorkIssueChangeLogFieldResolution
			val.Set.Resolution = sdk.StringPointer(change.ToString)
		case "issuetype":
			field = sdk.WorkIssueChangeLogFieldType
			val.Set.Type = &sdk.NameID{
				Name: sdk.StringPointer(change.ToString),
				ID:   sdk.StringPointer(sdk.NewWorkIssueTypeID(customerID, refType, change.To)),
			}
		case "project":
			field = sdk.WorkIssueChangeLogFieldProjectID
			projectID := sdk.NewWorkProjectID(customerID, change.To, refType)
			val.Set.ProjectID = &projectID
		case "Key":
			field = sdk.WorkIssueChangeLogFieldIdentifier
			val.Set.Identifier = sdk.StringPointer(change.ToString)
			change.To = change.ToString // to is null
		case "Sprint":
			field = sdk.WorkIssueChangeLogFieldSprintIds
			sprintID := []string{sdk.NewAgileSprintID(customerID, change.To, refType)}
			val.Set.SprintIDs = &sprintID
		case "duedate":
			field = sdk.WorkIssueChangeLogFieldDueDate
			if change.To == "" {
				val.Unset.DueDate = sdk.BoolPointer(true)
			} else {
				t, err := time.Parse("2006-01-02", change.To)
				if err != nil {
					return fmt.Errorf("error parsing due date time: %w", err)
				}
				val.Set.DueDate = &t
			}
		}
		// TODO: find a way to replicate "PARENT_ID" webhook
		if !skip {
			changeItem := sdk.WorkIssueChangeLog{
				RefID:      changelog.Changelog.ID,
				Field:      field,
				From:       change.From,
				FromString: change.FromString,
				To:         change.To,
				ToString:   change.ToString,
				UserID:     changelog.User.RefID(),
				Ordinal:    changelog.Timestamp + int64(i),
			}
			sdk.ConvertTimeToDateModel(ts, &changeItem.CreatedDate)
			if val.Push.ChangeLogs == nil {
				l := make([]sdk.WorkIssueChangeLog, 0)
				val.Push.ChangeLogs = &l
			}
			cl := append(*val.Push.ChangeLogs, changeItem)
			val.Push.ChangeLogs = &cl
		}
	}
	update := sdk.NewWorkIssueUpdate(customerID, integrationInstanceID, changelog.Issue.ID, refType, val)
	sdk.LogDebug(i.logger, "sending issue update", "data", sdk.Stringify(update))
	if err := pipe.Write(update); err != nil {
		return fmt.Errorf("error writing issue update to pipe: %w", err)
	}

	if updatedStatus {
		ts := time.Now()
		authCfg, err := i.createAuthConfig(webhook)
		if err != nil {
			return fmt.Errorf("error creating authconfig: %w", err)
		}
		api := newAgileAPI(i.logger, authCfg, customerID, integrationInstanceID, i.httpmanager)
		projectID := sdk.NewWorkProjectID(customerID, changelog.Issue.Fields.Project.ID, refType)
		sdk.LogDebug(i.logger, "updating board for issue", "issue", changelog.Issue.ID)
		if err := updateIssueBoards(webhook.State(), pipe, api, customerID, integrationInstanceID, changelog.Issue.Key, projectID); err != nil {
			return fmt.Errorf("error sending updated issue boards: %w", err)
		}
		sdk.LogDebug(i.logger, "done processing boards for issue", "issue", changelog.Issue.ID, "duration", time.Since(ts))
	}
	return nil
}

func updateIssueBoards(state sdk.State, pipe sdk.Pipe, api *agileAPI, customerID, integrationInstanceID, issueKey, projectID string) error {
	if api.authConfig.SupportsAgileAPI {
		boards, err := findBoardsForIssueInProject(state, api, issueKey, projectID)
		if err != nil {
			return fmt.Errorf("error finding boards for issue: %w", err)
		}
		if len(boards) == 0 {
			// if issue is on no project boards then search all boards
			boards, err = findBoardsForIssue(state, api, issueKey, nil)
			if err != nil {
				return fmt.Errorf("error finding boards for issue: %w", err)
			}
		}
		for _, boardID := range boards {
			// re-export boards
			if err := fetchAndExportBoard(api, state, pipe, customerID, integrationInstanceID, boardID); err != nil {
				return fmt.Errorf("error updating board for issue: %w", err)
			}
		}
	}
	return nil
}

func (i *JiraIntegration) webhookCreateIssue(webhook sdk.WebHook, rawdata []byte, pipe sdk.Pipe) error {
	// since we need transitions we have to go fetch the whole object from jira 😢
	var created struct {
		Issue struct {
			ID string `json:"id"`
		} `json:"issue"`
	}
	if err := json.Unmarshal(rawdata, &created); err != nil {
		return fmt.Errorf("error parsing json for changelog: %w", err)
	}
	sdk.LogDebug(i.logger, "new issue webhook received", "issue", created.Issue.ID)

	stats := &stats{}
	stats.started = time.Now()
	authConfig, err := i.createAuthConfig(webhook)
	if err != nil {
		return fmt.Errorf("error creating auth config: %w", err)
	}
	state := i.newState(i.logger, pipe, authConfig, webhook.Config(), false, webhook.IntegrationInstanceID())
	customfields, err := i.fetchCustomFields(i.logger, state.export, webhook.CustomerID(), state.authConfig)
	if err != nil {
		return err
	}
	sprintMgr := newSprintManager(webhook.CustomerID(), pipe, stats, webhook.IntegrationInstanceID(), state.authConfig.SupportsAgileAPI)
	userMgr := newUserManager(webhook.CustomerID(), state.authConfig.WebsiteURL, pipe, stats, webhook.IntegrationInstanceID())
	mgr := newIssueIDManager(i.logger, i, webhook, pipe, sprintMgr, userMgr, customfields, state.authConfig, stats)
	issue, comments, err := mgr.fetchIssue(created.Issue.ID, false)
	if err != nil {
		return fmt.Errorf("error fetching issue: %w", err)
	}
	if issue == nil {
		sdk.LogDebug(i.logger, "unable to find issue for webhook", "issue", created.Issue.ID, "customer_id", webhook.CustomerID())
		return nil
	}
	sdk.LogDebug(i.logger, "sending new issue", "data", issue.Stringify())
	if err := pipe.Write(issue); err != nil {
		return err
	}
	for _, comment := range comments {
		if err := pipe.Write(comment); err != nil {
			return err
		}
	}
	if state.authConfig.SupportsAgileAPI {
		ts := time.Now()
		sdk.LogDebug(i.logger, "updating board for issue", "issue", issue.ID)
		api := newAgileAPI(i.logger, state.authConfig, webhook.CustomerID(), webhook.IntegrationInstanceID(), i.httpmanager)
		if err := updateIssueBoards(webhook.State(), pipe, api, webhook.CustomerID(), webhook.IntegrationInstanceID(), issue.Identifier, issue.ProjectID); err != nil {
			return fmt.Errorf("error re-exporting board: %w", err)
		}
		sdk.LogDebug(i.logger, "done processing boards for issue", "issue", issue.ID, "duration", time.Since(ts))
	}
	return nil
}

func (i *JiraIntegration) webhookDeleteIssue(customerID string, integrationInstanceID string, rawdata []byte, pipe sdk.Pipe) error {
	var deleted struct {
		User  user `json:"user"`
		Issue struct {
			ID string `json:"id"`
		} `json:"issue"`
	}
	if err := json.Unmarshal(rawdata, &deleted); err != nil {
		return fmt.Errorf("error parsing json for deletion: %w", err)
	}
	sdk.LogDebug(i.logger, "deleted issue webhook received", "issue", deleted.Issue.ID)
	val := sdk.WorkIssueUpdate{}
	active := false
	val.Set.Active = &active
	update := sdk.NewWorkIssueUpdate(customerID, integrationInstanceID, deleted.Issue.ID, refType, val)
	sdk.LogDebug(i.logger, "deleting issue", "issue", deleted.Issue.ID)
	return pipe.Write(update)
}

// webhookUpsertComment will work for comment_created and comment_updated since they both require refetch because they dont deliver adf!
func (i *JiraIntegration) webhookUpsertComment(webhook sdk.WebHook) error {
	pipe := webhook.Pipe()
	customerID := webhook.CustomerID()
	integrationInstanceID := webhook.IntegrationInstanceID()
	var created struct {
		Timestamp int64 `json:"timestamp"`
		Comment   struct {
			ID string `json:"id"`
		} `json:"comment"`
		Issue struct {
			ID     string `json:"id"`
			Key    string `json:"key"`
			Fields struct {
				Project struct {
					ID string `json:"id"`
				} `json:"project"`
			} `json:"fields"`
		} `json:"issue"`
	}
	if err := json.Unmarshal(webhook.Bytes(), &created); err != nil {
		return fmt.Errorf("error parsing json for comment: %w", err)
	}
	sdk.LogDebug(i.logger, "new comment webhook recieved", "comment", created.Comment.ID)
	authcfg, err := i.createAuthConfig(webhook)
	if err != nil {
		return fmt.Errorf("error creating authconfig: %w", err)
	}
	um := newUserManager(customerID, authcfg.WebsiteURL, pipe, nil, integrationInstanceID)
	// TODO(robin): make a CommentManager interface that we pass in instead
	comment, err := i.fetchComment(authcfg, um, integrationInstanceID, customerID, created.Issue.ID, created.Issue.Key, created.Comment.ID, created.Issue.Fields.Project.ID)
	if err != nil {
		return fmt.Errorf("error getting comment: %w", err)
	}
	sdk.LogDebug(i.logger, "sending new comment", "data", sdk.Stringify(comment))
	return pipe.Write(comment)
}

func (i *JiraIntegration) webhookDeleteComment(customerID string, integrationInstanceID string, rawdata []byte, pipe sdk.Pipe) error {
	var deleted struct {
		User    user `json:"user"`
		Comment struct {
			ID string `json:"id"`
		} `json:"comment"`
		Issue struct {
			ID     string `json:"id"`
			Key    string `json:"key"`
			Fields struct {
				Project struct {
					ID string `json:"id"`
				} `json:"project"`
			} `json:"fields"`
		} `json:"issue"`
	}
	if err := json.Unmarshal(rawdata, &deleted); err != nil {
		return fmt.Errorf("error parsing json for deletion: %w", err)
	}
	sdk.LogDebug(i.logger, "deleted Comment webhook received", "Comment", deleted.Comment.ID)
	val := sdk.WorkIssueCommentUpdate{}
	active := false
	val.Set.Active = &active
	update := sdk.NewWorkIssueCommentUpdate(customerID, integrationInstanceID, deleted.Comment.ID, refType, deleted.Issue.Fields.Project.ID, val)
	sdk.LogDebug(i.logger, "deleting Comment", "comment", deleted.Comment.ID)
	return pipe.Write(update)
}

// webhookUpsertProject will work for project_created and project_updated since they both require refetch.
func (i *JiraIntegration) webhookUpsertProject(webhook sdk.WebHook, pipe sdk.Pipe) error {
	var upsert struct {
		Timestamp int64 `json:"timestamp"`
		Project   struct {
			ID  int64  `json:"id"`
			Key string `json:"key"`
		} `json:"project"`
	}
	if err := json.Unmarshal(webhook.Bytes(), &upsert); err != nil {
		return fmt.Errorf("error parsing json for created project: %w", err)
	}
	// jira webhooks (sometimes) return ids as numbers instead of strings
	refID := fmt.Sprintf("%d", upsert.Project.ID)
	authConfig, err := i.createAuthConfig(webhook)
	if err != nil {
		return fmt.Errorf("error creating auth config: %w", err)
	}
	state := i.newState(i.logger, pipe, authConfig, webhook.Config(), false, webhook.IntegrationInstanceID())
	sdk.LogDebug(i.logger, "new project webhook received", "project", refID)
	project, err := i.fetchProject(state, webhook.CustomerID(), refID)
	if err != nil {
		return err
	}
	if project == nil {
		sdk.LogDebug(i.logger, "unable to find new project", "project", refID)
		return nil
	}
	sdk.LogDebug(i.logger, "sending project", "data", sdk.Stringify(project))
	return pipe.Write(project)
}

func (i *JiraIntegration) webhookDeleteProject(customerID string, integrationInstanceID string, rawdata []byte, pipe sdk.Pipe) error {
	var deleted struct {
		Timestamp int64 `json:"timestamp"`
		Project   struct {
			ID int64 `json:"id"`
		} `json:"project"`
	}
	if err := json.Unmarshal(rawdata, &deleted); err != nil {
		return fmt.Errorf("error parsing json for deletion: %w", err)
	}
	refid := fmt.Sprintf("%d", deleted.Project.ID)
	sdk.LogDebug(i.logger, "deleted Project webhook received", "Project", refid)
	val := sdk.WorkProjectUpdate{}
	active := false
	val.Set.Active = &active
	update := sdk.NewWorkProjectUpdate(customerID, integrationInstanceID, refid, refType, val)
	sdk.LogDebug(i.logger, "deleting Project", "project", refid)
	return pipe.Write(update)
}

func (i *JiraIntegration) webhookUpsertUser(webhook sdk.WebHook) error {
	authConfig, err := i.createAuthConfig(webhook)
	if err != nil {
		return fmt.Errorf("error creating auth config for webhook: %w", err)
	}
	um := newUserManager(webhook.CustomerID(), authConfig.WebsiteURL, webhook.Pipe(), nil, webhook.IntegrationInstanceID())
	return webhookUpsertUser(i.logger, um, webhook.Bytes())
}

func webhookUpsertUser(logger sdk.Logger, userManager UserManager, rawdata []byte) error {
	// TODO(robin): test on premise
	var upserted struct {
		Timestamp int64 `json:"timestamp"`
		User      user  `json:"user"`
	}
	if err := json.Unmarshal(rawdata, &upserted); err != nil {
		return fmt.Errorf("error parsing json for user: %w", err)
	}
	sdk.LogDebug(logger, "upserting user")
	return userManager.Emit(upserted.User)
}

func (i *JiraIntegration) webhookCreateSprint(webhook sdk.WebHook) error {
	authConfig, err := i.createAuthConfig(webhook)
	if err != nil {
		return fmt.Errorf("error creating auth config for webhook: %w", err)
	}
	api := newAgileAPI(i.logger, authConfig, webhook.CustomerID(), webhook.IntegrationInstanceID(), i.httpmanager)
	return webhookCreateSprint(api, webhook.Bytes(), webhook.Pipe())
}

func webhookCreateSprint(api *agileAPI, rawdata []byte, pipe sdk.Pipe) error {
	var created struct {
		Timestamp int64 `json:"timestamp"`
		Sprint    struct {
			ID            int `json:"id"`
			OriginBoardID int `json:"originBoardId"`
		} `json:"sprint"`
	}
	if err := json.Unmarshal(rawdata, &created); err != nil {
		return fmt.Errorf("error parsing json for created project: %w", err)
	}
	sprint, err := api.fetchOneSprint(created.Sprint.ID, created.Sprint.OriginBoardID)
	if err != nil {
		return fmt.Errorf("error fetching sprint: %w", err)
	}
	return pipe.Write(sprint)
}

func (i *JiraIntegration) webhookDeleteSprint(customerID string, integrationInstanceID string, rawdata []byte, pipe sdk.Pipe) error {
	var deleted struct {
		Timestamp int64 `json:"timestamp"`
		Sprint    struct {
			ID            int `json:"id"`
			OriginBoardID int `json:"originBoardId"`
		} `json:"sprint"`
	}
	if err := json.Unmarshal(rawdata, &deleted); err != nil {
		return fmt.Errorf("error parsing json for created project: %w", err)
	}
	refid := strconv.Itoa(deleted.Sprint.ID)
	val := sdk.AgileSprintUpdate{}
	active := false
	val.Set.Active = &active
	update := sdk.NewAgileSprintUpdate(customerID, integrationInstanceID, refid, refType, val)
	sdk.LogDebug(i.logger, "deleting sprint", "sprint", refid)
	return pipe.Write(update)
}

type sprintProjection struct {
	ID           int        `json:"id"`
	Self         string     `json:"self"`
	Name         string     `json:"name"`
	State        string     `json:"state"`
	StartDate    *time.Time `json:"startDate,omitempty"`
	CompleteDate *time.Time `json:"completeDate"`
	EndDate      *time.Time `json:"endDate,omitempty"`
	Goal         *string    `json:"goal,omitempty"`
}

func buildSprintUpdate(old, new sprintProjection) (sdk.AgileSprintUpdate, bool) {
	val := sdk.AgileSprintUpdate{}
	var setName, setStatus, setGoal, setStartDate, setEndDate bool

	// check name differences
	if new.Name != old.Name {
		setName = true
	}
	if setName {
		val.Set.Name = &(new.Name)
	}

	// check state differences
	if new.State != old.State {
		setStatus = true
	}
	if setStatus {
		status := sprintStateMap[new.State]
		val.Set.Status = &status
	}

	// check goal differences
	if (new.Goal != nil && old.Goal != nil && *new.Goal != *old.Goal) || // if goal changed
		(new.Goal != nil && old.Goal == nil) { // if goal was set
		setGoal = true
	}
	if setGoal {
		val.Set.Goal = new.Goal
	}

	// check startdate differences
	if (new.StartDate != nil && old.StartDate != nil && *new.StartDate != *old.StartDate) || // if start date changed
		(new.StartDate != nil && old.StartDate == nil) { // if start date was set
		setStartDate = true
	}
	if setStartDate {
		val.Set.StartedDate = new.StartDate
	}

	// check enddate differences
	if (new.EndDate != nil && old.EndDate != nil && *new.EndDate != *old.EndDate) || // if end date changed
		(new.EndDate != nil && old.EndDate == nil) { // if end date was set
		setEndDate = true
	}
	if setEndDate {
		val.Set.EndedDate = new.EndDate
	}
	return val, setName || setStatus || setGoal || setStartDate || setEndDate
}

func (i *JiraIntegration) webhookUpdateSprint(customerID string, integrationInstanceID string, rawdata []byte, pipe sdk.Pipe) error {
	var updated struct {
		Timestamp int64            `json:"timestamp"`
		Sprint    sprintProjection `json:"sprint"`
		OldValue  sprintProjection `json:"oldValue"`
	}
	if err := json.Unmarshal(rawdata, &updated); err != nil {
		return fmt.Errorf("error parsing json for created project: %w", err)
	}
	refid := strconv.Itoa(updated.Sprint.ID)
	val, change := buildSprintUpdate(updated.OldValue, updated.Sprint)
	if !change {
		sdk.LogDebug(i.logger, "no changes to sprint from webhook", "sprint", refid)
		return nil
	}
	update := sdk.NewAgileSprintUpdate(customerID, integrationInstanceID, refid, refType, val)
	sdk.LogDebug(i.logger, "updating sprint", "sprint", refid)
	return pipe.Write(update)
}

func (i *JiraIntegration) webhookCloseSprint(customerID string, integrationInstanceID string, rawdata []byte, pipe sdk.Pipe) error {
	var closed struct {
		Timestamp int64            `json:"timestamp"`
		Sprint    sprintProjection `json:"sprint"`
	}
	if err := json.Unmarshal(rawdata, &closed); err != nil {
		return fmt.Errorf("error parsing json for created project: %w", err)
	}
	refid := strconv.Itoa(closed.Sprint.ID)
	val := sdk.AgileSprintUpdate{}
	status := sdk.AgileSprintStatusClosed
	val.Set.Status = &status
	if closed.Sprint.CompleteDate == nil {
		sdk.LogWarn(i.logger, "closed sprint had no completed date, using now", "sprint", refid)
		now := time.Now()
		closed.Sprint.CompleteDate = &now
	}
	val.Set.CompletedDate = closed.Sprint.CompleteDate
	update := sdk.NewAgileSprintUpdate(customerID, integrationInstanceID, refid, refType, val)
	sdk.LogDebug(i.logger, "updating sprint", "sprint", refid)
	return pipe.Write(update)
}

func (i *JiraIntegration) webhookCreateBoard(webhook sdk.WebHook) error {
	pipe := webhook.Pipe()
	customerID := webhook.CustomerID()
	integrationInstanceID := webhook.IntegrationInstanceID()

	var update struct {
		Timestamp int64 `json:"timestamp"`
		Board     struct {
			ID int `json:"id"`
		} `json:"board"`
	}
	if err := json.Unmarshal(webhook.Bytes(), &update); err != nil {
		return fmt.Errorf("error parsing json for created project: %w", err)
	}
	refid := strconv.Itoa(update.Board.ID)
	authConfig, err := i.createAuthConfig(webhook)
	if err != nil {
		return fmt.Errorf("error getting auth config: %w", err)
	}
	api := newAgileAPI(i.logger, authConfig, customerID, integrationInstanceID, i.httpmanager)
	return fetchAndExportBoard(api, webhook.State(), pipe, customerID, integrationInstanceID, refid)
}

func (i *JiraIntegration) webhookUpdateBoard(customerID string, integrationInstanceID string, rawdata []byte, pipe sdk.Pipe) error {
	var update struct {
		Timestamp int64 `json:"timestamp"`
		Board     struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
			Type string `json:"type"` // TODO(robin): investigate if you can change board types
		} `json:"board"`
	}
	if err := json.Unmarshal(rawdata, &update); err != nil {
		return fmt.Errorf("error parsing json for created project: %w", err)
	}
	refid := strconv.Itoa(update.Board.ID)
	val := sdk.AgileBoardUpdate{}
	val.Set.Name = &(update.Board.Name)
	model := sdk.NewAgileBoardUpdate(customerID, integrationInstanceID, refid, refType, val)
	return pipe.Write(model)
}

func (i *JiraIntegration) webhookDeleteBoard(customerID string, integrationInstanceID string, rawdata []byte, pipe sdk.Pipe) error {
	var update struct {
		Timestamp int64 `json:"timestamp"`
		Board     struct {
			ID int `json:"id"`
		} `json:"board"`
	}
	if err := json.Unmarshal(rawdata, &update); err != nil {
		return fmt.Errorf("error parsing json for created project: %w", err)
	}
	refid := strconv.Itoa(update.Board.ID)
	val := sdk.AgileBoardUpdate{}
	active := false
	val.Set.Active = &active
	model := sdk.NewAgileBoardUpdate(customerID, integrationInstanceID, refid, refType, val)
	return pipe.Write(model)
}

func makeLinks(issueID string, linkRefID string, linkType sdk.WorkIssueLinkedIssuesLinkType, reverse bool) *[]sdk.WorkIssueLinkedIssues {
	links := []sdk.WorkIssueLinkedIssues{
		{
			IssueID:          issueID,
			LinkType:         linkType,
			RefID:            linkRefID,
			ReverseDirection: reverse,
		},
	}
	return &links
}

var linkTypeMap = map[string]sdk.WorkIssueLinkedIssuesLinkType{
	"Relates":          sdk.WorkIssueLinkedIssuesLinkTypeRelates,
	"Blocks":           sdk.WorkIssueLinkedIssuesLinkTypeBlocks,
	"Cloners":          sdk.WorkIssueLinkedIssuesLinkTypeClones,
	"Duplicate":        sdk.WorkIssueLinkedIssuesLinkTypeDuplicates,
	"Problem/Incident": sdk.WorkIssueLinkedIssuesLinkTypeCauses,
}

func (i *JiraIntegration) webhookIssueLinkCreated(customerID string, integrationInstanceID string, rawdata []byte, pipe sdk.Pipe) error {
	return webhookHandleIssueLink(i.logger, customerID, integrationInstanceID, rawdata, pipe, false)
}

func (i *JiraIntegration) webhookIssueLinkDeleted(customerID string, integrationInstanceID string, rawdata []byte, pipe sdk.Pipe) error {
	return webhookHandleIssueLink(i.logger, customerID, integrationInstanceID, rawdata, pipe, true)
}

func webhookHandleIssueLink(logger sdk.Logger, customerID string, integrationInstanceID string, rawdata []byte, pipe sdk.Pipe, delete bool) error {
	var link struct {
		Timestamp int64 `json:"timestamp"`
		IssueLink struct {
			ID                 int `json:"id"`
			SourceIssueID      int `json:"sourceIssueId"`
			DestinationIssueID int `json:"destinationIssueId"`
			IssueLinkType      struct {
				ID                int    `json:"id"`
				Name              string `json:"name"`
				OutwardName       string `json:"outwardName"`
				InwardName        string `json:"inwardName"`
				IsSubTaskLinkType bool   `json:"isSubTaskLinkType"`
				IsSystemLinkType  bool   `json:"isSystemLinkType"`
			} `json:"issueLinkType"`
		} `json:"issueLink"`
	}
	if err := json.Unmarshal(rawdata, &link); err != nil {
		return fmt.Errorf("error parsing json for link: %w", err)
	}
	sourceRefID := strconv.Itoa(link.IssueLink.SourceIssueID)
	destRefID := strconv.Itoa(link.IssueLink.DestinationIssueID)
	sourceID := sdk.NewWorkIssueID(customerID, sourceRefID, refType)
	destID := sdk.NewWorkIssueID(customerID, destRefID, refType)
	issueLinkID := strconv.Itoa(link.IssueLink.ID)

	linkType, ok := linkTypeMap[link.IssueLink.IssueLinkType.Name]
	if !ok {
		if link.IssueLink.IssueLinkType.Name != "Epic-Story Link" {
			// "Epic-Story Link" //NOTE: changing an epic sends an issue webhook, so webhookUpdateIssue will handle this
			sdk.LogDebug(logger, "unhandled issue link", "type", link.IssueLink.IssueLinkType.Name)
		}
		return nil
	}
	var sourceUpdate, destUpdate sdk.WorkIssueUpdate
	// create link from source to dest
	source := makeLinks(destID, issueLinkID, linkType, false)
	// create reverse link from dest back to source
	dest := makeLinks(sourceID, issueLinkID, linkType, true)
	if delete {
		sourceUpdate.Pull.LinkedIssues = source
		destUpdate.Pull.LinkedIssues = dest
	} else {
		sourceUpdate.Push.LinkedIssues = source
		destUpdate.Push.LinkedIssues = dest
	}
	if err := pipe.Write(sdk.NewWorkIssueUpdate(customerID, integrationInstanceID, sourceRefID, refType, sourceUpdate)); err != nil {
		return fmt.Errorf("error writing update to pipe: %w", err)
	}
	if err := pipe.Write(sdk.NewWorkIssueUpdate(customerID, integrationInstanceID, destRefID, refType, destUpdate)); err != nil {
		return fmt.Errorf("error writing update to pipe: %w", err)
	}
	return nil
}

// WebHook is called when a webhook is received on behalf of the integration
func (i *JiraIntegration) WebHook(webhook sdk.WebHook) error {
	sdk.LogInfo(i.logger, "webhook request received", "customer_id", webhook.CustomerID())
	var event webhookEvent
	if err := event.UnmarshalJSON(webhook.Bytes()); err != nil {
		return fmt.Errorf("error parsing JSON event payload for webhook: %w", err)
	}
	customerID := webhook.CustomerID()
	integrationInstanceID := webhook.IntegrationInstanceID()
	pipe := webhook.Pipe()
	switch event.Event {
	case "jira:issue_updated":
		return i.webhookUpdateIssue(webhook)
	case "jira:issue_created":
		return i.webhookCreateIssue(webhook, webhook.Bytes(), pipe)
	case "jira:issue_deleted":
		return i.webhookDeleteIssue(customerID, integrationInstanceID, webhook.Bytes(), pipe)
	case "comment_created", "comment_updated":
		return i.webhookUpsertComment(webhook)
	case "comment_deleted":
		return i.webhookDeleteComment(customerID, integrationInstanceID, webhook.Bytes(), pipe)
	case "project_created", "project_updated":
		return i.webhookUpsertProject(webhook, pipe)
	case "project_deleted":
		return i.webhookDeleteProject(customerID, integrationInstanceID, webhook.Bytes(), pipe)
	case "user_created", "user_updated":
		return i.webhookUpsertUser(webhook)
	case "user_deleted":
		// TODO
	case "sprint_created":
		return i.webhookCreateSprint(webhook)
	case "sprint_deleted":
		return i.webhookDeleteSprint(customerID, integrationInstanceID, webhook.Bytes(), pipe)
	case "sprint_updated":
		return i.webhookUpdateSprint(customerID, integrationInstanceID, webhook.Bytes(), pipe)
	case "sprint_started":
		// NOTE: jira sends a sprint_updated on sprint start with all the info we need.
		// Strangely, it does not send the same for sprint_closed.
	case "sprint_closed":
		return i.webhookCloseSprint(customerID, integrationInstanceID, webhook.Bytes(), pipe)
	case "board_created", "board_configuration_changed":
		return i.webhookCreateBoard(webhook)
	case "board_updated":
		return i.webhookUpdateBoard(customerID, integrationInstanceID, webhook.Bytes(), pipe)
	case "board_deleted":
		return i.webhookUpdateBoard(customerID, integrationInstanceID, webhook.Bytes(), pipe)
	case "issuelink_created":
		return i.webhookIssueLinkCreated(customerID, integrationInstanceID, webhook.Bytes(), pipe)
	case "issuelink_deleted":
		return i.webhookIssueLinkDeleted(customerID, integrationInstanceID, webhook.Bytes(), pipe)
	default:
		sdk.LogDebug(i.logger, "webhook event not handled", "event", event.Event, "payload", string(webhook.Bytes()))
	}
	return nil
}
