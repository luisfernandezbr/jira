package internal

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/pinpt/agent.next/sdk"
)

const (
	// ValidateURL will check that a jira url is reachable
	ValidateURL = "VALIDATE_URL"
	// FetchAccounts will fetch accounts
	FetchAccounts = "FETCH_ACCOUNTS"
)

type projectSearchResult struct {
	MaxResults int  `json:"maxResults"`
	StartAt    int  `json:"startAt"`
	Total      int  `json:"total"`
	IsLast     bool `json:"isLast"`
}

// type jiraOrg struct {
// 	ID   string
// 	Type string
// }

type validateAccount struct {
	ID          string
	Name        string
	Description string
	AvatarURL   string
	TotalCount  int
	Type        string
	Public      bool
}

// Validate will perform pre-installation operations on behalf of the UI
func (i *JiraIntegration) Validate(validate sdk.Validate) (map[string]interface{}, error) {
	config := validate.Config()
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
	case FetchAccounts:
		authConfig, err := i.createAuthConfig(validate)
		if err != nil {
			return nil, fmt.Errorf("error creating auth config: %w", err)
		}
		projectURL := sdk.JoinURL(authConfig.APIURL, "/rest/api/3/project/search")
		client := i.httpmanager.New(projectURL, nil)
		qs := make(url.Values)
		qs.Set("maxResults", "1") // NOTE: We just need the total, this would be 0, but 1 is the minimum value.
		qs.Set("status", "live")
		qs.Set("typeKey", "software")
		var resp projectSearchResult
		r, err := client.Get(&resp, append(authConfig.Middleware, sdk.WithGetQueryParameters(qs))...)
		if err != nil {
			if httperr, ok := err.(*sdk.HTTPError); ok {
				buf, _ := ioutil.ReadAll(httperr.Body)
				fmt.Println("AAaaaa", string(buf))
			}
			return nil, fmt.Errorf("error fetching project accounts: %w", err)
		}
		if r.StatusCode != http.StatusOK {
			sdk.LogDebug(i.logger, "unusual status code", "code", r.StatusCode)
		}
		acc := validateAccount{
			ID:         "1",
			TotalCount: resp.Total,
		}
		// TODO(robin): get account info
		// orgurl := sdk.JoinURL(authConfig.APIURL, "/admin/v1/orgs/pinpt-hq")
		// client = i.httpmanager.New(orgurl, nil)
		// fmt.Println("the url", orgurl)
		// qq := make(map[string]interface{})
		// r, err = client.Get(&qq, authConfig.Middleware...)
		// if err != nil {
		// 	return nil, fmt.Errorf("error fetching org: %w", err)
		// }
		// fmt.Println(">>>", sdk.Stringify(qq))
		return map[string]interface{}{
			"accounts": acc,
		}, nil
	default:
		return nil, fmt.Errorf("unknown action %s", action)
	}
}
