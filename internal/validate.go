package internal

import (
	"fmt"

	"github.com/pinpt/agent.next/sdk"
)

const (
	// ValidateURL will check that a jira url is reachable
	ValidateURL = "VALIDATE_URL"
	// FetchAccounts will fetch accounts
	FetchAccounts = "FETCH_ACCOUNTS"
)

// Validate will perform pre-installation operations on behalf of the UI
func (i *JiraIntegration) Validate(config sdk.Config) (map[string]interface{}, error) {
	sdk.LogDebug(i.logger, "Validation", "config", config)
	found, action := config.GetString("action")
	if !found {
		return nil, fmt.Errorf("validation had no action")
	}
	switch action {
	case ValidateURL:
		found, url := config.GetString("url")
		if !found {
			return nil, fmt.Errorf("url validation had no url")
		}
		client := i.httpmanager.New(url, nil)
		_, err := client.Get(nil)
		if err != nil {
			if _, ok := err.(*sdk.HTTPError); ok {
				// NOTE: if we get an http response then we're good
				// TODO(robin): scrape err body for jira metas
				return nil, nil
			}
			return nil, fmt.Errorf("error reaching %s: %w", url, err)
		}
		return nil, nil
	default:
		return nil, fmt.Errorf("unknown action %s", action)
	}
}
