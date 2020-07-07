package internal

import (
	"encoding/json"
	"fmt"

	"github.com/pinpt/agent.next/sdk"
)

const webhookStateKey = "webhook"

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

func (i *JiraIntegration) uninstallWebHookIfNecessary(logger sdk.Logger, config sdk.Config, state sdk.State, authConfig authConfig, customerID string, integrationInstanceID string) error {
	if config.BasicAuth == nil {
		sdk.LogInfo(logger, "skipping web hook uninstall since not using basic auth")
	}
	var hookurl string
	if ok, _ := state.Get(webhookStateKey, &hookurl); ok {
		client := i.httpmanager.New(hookurl, nil)
		var res interface{}
		if _, err := client.Delete(&res, authConfig.Middleware...); err != nil {
			return err
		}
		if err := state.Delete(webhookStateKey); err != nil {
			return err
		}
		sdk.LogInfo(logger, "web hook removed", "url", hookurl)
	}
	return nil
}

func (i *JiraIntegration) installWebHookIfNecessary(logger sdk.Logger, config sdk.Config, state sdk.State, authConfig authConfig, customerID string, integrationInstanceID string) error {
	if config.BasicAuth == nil {
		sdk.LogInfo(logger, "skipping web hook install since not using basic auth")
		return nil
	}
	if state.Exists(webhookStateKey) {
		sdk.LogInfo(logger, "skipping web hook install since already installed")
		return nil
	}
	webhookurl, err := i.manager.CreateWebHook(customerID, refType, integrationInstanceID, "")
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
		return err
	}
	sdk.LogInfo(logger, "installed webhook", "id", res.Self)
	return state.Set(webhookStateKey, res.Self)
}

type webhookEvent struct {
	Event string `json:"webhookEvent"`
}

func (i *JiraIntegration) upsertIssue(customerID string, integrationInstanceID string, rawdata []byte, pipe sdk.Pipe) error {
	var changelog struct {
		Issue struct {
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
	val := sdk.WorkIssueUpsert{}
	for _, change := range changelog.Changelog.Items {
		switch change.Field {
		case "summary":
			val.Set.Title = sdk.StringPointer(change.ToString)
		case "description":
			val.Set.Description = sdk.StringPointer(change.ToString)
		case "status":
			val.Set.Status = &sdk.NameID{
				Name: sdk.StringPointer(change.ToString),
				ID:   sdk.StringPointer(sdk.NewWorkIssueStatusID(customerID, refType, change.To)),
			}
		}
	}
	upsert := sdk.NewWorkIssueUpsert(customerID, integrationInstanceID, changelog.Issue.ID, refType, val)
	sdk.LogDebug(i.logger, "sending issue upsert", "data", sdk.Stringify(upsert))
	return pipe.Write(upsert)
}

// WebHook is called when a webhook is received on behalf of the integration
func (i *JiraIntegration) WebHook(webhook sdk.WebHook) error {
	sdk.LogInfo(i.logger, "webhook request received", "customer_id", webhook.CustomerID())
	fmt.Println(string(webhook.Bytes()))
	var event webhookEvent
	if err := event.UnmarshalJSON(webhook.Bytes()); err != nil {
		return fmt.Errorf("error parsing JSON event payload for webhook: %w", err)
	}
	customerID := webhook.CustomerID()
	integrationInstanceID := webhook.IntegrationInstanceID()
	pipe := webhook.Pipe()
	switch event.Event {
	case "jira:issue_updated":
		return i.upsertIssue(customerID, integrationInstanceID, webhook.Bytes(), pipe)
	default:
		sdk.LogDebug(i.logger, "webhook event not handled", "event", event.Event, "payload", string(webhook.Bytes()))
	}
	return nil
}
