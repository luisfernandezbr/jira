package internal

import (
	"encoding/json"
	"fmt"
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

func (i *JiraIntegration) webhookUpdateIssue(customerID string, integrationInstanceID string, rawdata []byte, pipe sdk.Pipe) error {
	var changelog struct {
		Timestamp int64 `json:"timestamp"`
		User      user  `json:"user"`
		Issue     struct {
			ID string `json:"id"`
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
		case "Epic Link":
			field = sdk.WorkIssueChangeLogFieldEpicID
			if change.To == "" {
				val.Unset.EpicID = sdk.BoolPointer(true)
			} else {
				val.Set.EpicID = sdk.StringPointer(sdk.NewWorkIssueID(customerID, change.To, refType))
			}
		}
		// TODO:
		// "ASSIGNEE_REF_ID"
		// "DUE_DATE"
		// "IDENTIFIER"
		// "PARENT_ID"
		// "PRIORITY"
		// "PROJECT_ID"
		// "REPORTER_REF_ID"
		// "RESOLUTION"
		// "SPRINT_IDS"
		// "TAGS"
		// "TYPE"
		if !skip {
			change := sdk.WorkIssueChangeLog{
				RefID:      changelog.Changelog.ID,
				Field:      field,
				From:       changelog.Changelog.Items[0].From,
				FromString: changelog.Changelog.Items[0].FromString,
				To:         changelog.Changelog.Items[0].To,
				ToString:   changelog.Changelog.Items[0].ToString,
				UserID:     changelog.User.RefID(),
				Ordinal:    changelog.Timestamp + int64(i),
			}
			sdk.ConvertTimeToDateModel(ts, &change.CreatedDate)
			if val.Push.ChangeLogs == nil {
				l := make([]sdk.WorkIssueChangeLog, 0)
				val.Push.ChangeLogs = &l
			}
			cl := append(*val.Push.ChangeLogs, change)
			val.Push.ChangeLogs = &cl
		}
	}
	update := sdk.NewWorkIssueUpdate(customerID, integrationInstanceID, changelog.Issue.ID, refType, val)
	sdk.LogDebug(i.logger, "sending issue update", "data", sdk.Stringify(update))
	return pipe.Write(update)
}

func (i *JiraIntegration) webhookCreateIssue(webhook sdk.WebHook, rawdata []byte, pipe sdk.Pipe) error {
	// since we need transitions we have to go fetch the whole object from jira 😢
	var created struct {
		Issue struct {
			ID string `json:"id"`
		}
	}
	if err := json.Unmarshal(rawdata, &created); err != nil {
		return fmt.Errorf("error parsing json for changelog: %w", err)
	}
	sdk.LogDebug(i.logger, "new issue webhook received", "issue", created.Issue.ID)

	stats := &stats{}
	stats.started = time.Now()
	state, err := i.newState(i.logger, pipe, webhook.Config(), false, webhook.IntegrationInstanceID())
	if err != nil {
		return fmt.Errorf("unable to get state: %w", err)
	}
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
	return nil
}

func (i *JiraIntegration) webhookDeleteIssue(customerID string, integrationInstanceID string, rawdata []byte, pipe sdk.Pipe) error {
	var deleted struct {
		User  user `json:"user"`
		Issue struct {
			ID string `json:"id"`
		}
	}
	if err := json.Unmarshal(rawdata, &deleted); err != nil {
		return fmt.Errorf("error parsing json for deletion: %w", err)
	}
	sdk.LogDebug(i.logger, "new issue webhook received", "issue", deleted.Issue.ID)
	val := sdk.WorkIssueUpdate{}
	active := false
	val.Set.Active = &active
	update := sdk.NewWorkIssueUpdate(customerID, integrationInstanceID, deleted.Issue.ID, refType, val)
	sdk.LogDebug(i.logger, "sending issue update", "data", sdk.Stringify(update))
	return pipe.Write(update)
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
		return i.webhookUpdateIssue(customerID, integrationInstanceID, webhook.Bytes(), pipe)
	case "jira:issue_created":
		return i.webhookCreateIssue(webhook, webhook.Bytes(), pipe)
	case "jira:issue_deleted":
		return i.webhookDeleteIssue(customerID, integrationInstanceID, webhook.Bytes(), pipe)
	default:
		sdk.LogDebug(i.logger, "webhook event not handled", "event", event.Event, "payload", string(webhook.Bytes()))
	}
	return nil
}
