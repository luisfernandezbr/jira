package internal

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strings"

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

// Validate will perform pre-installation operations on behalf of the UI
func (i *JiraIntegration) Validate(validate sdk.Validate) (map[string]interface{}, error) {
	config := validate.Config()
	sdk.LogDebug(i.logger, "Validate", "config", config)
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
				sdk.LogError(i.logger, "error reading data for validate", "buf", string(buf))
			}
			return nil, fmt.Errorf("error fetching project accounts: %w", err)
		}
		if r.StatusCode != http.StatusOK {
			sdk.LogDebug(i.logger, "unusual status code", "code", r.StatusCode)
		}
		indexURL := sdk.JoinURL(authConfig.APIURL, "/")
		client = i.httpmanager.New(indexURL, nil)
		var resp2 interface{}
		r, err = client.Get(&resp2, authConfig.Middleware...)
		if err != nil {
			if httperr, ok := err.(*sdk.HTTPError); ok {
				buf, _ := ioutil.ReadAll(httperr.Body)
				sdk.LogError(i.logger, "error reading data for validate (index)", "buf", string(buf))
			}
			return nil, fmt.Errorf("error fetching project accounts: %w", err)
		}
		// try and extract the name if possible
		re := regexp.MustCompile(`<meta name="ajs-cloud-name" content="(.*?)">`)
		nametok := re.FindStringSubmatch(string(r.Body))
		var name string
		if len(nametok) > 0 {
			name = nametok[1]
		}
		if name == "" {
			i := strings.Index(authConfig.APIURL, "://")
			tok := strings.Split(authConfig.APIURL[i+3:], ".")
			name = tok[0]
		}
		re = regexp.MustCompile(`<link rel="shortcut icon" href="(.*?)">`)
		var avatar string
		avatartok := re.FindStringSubmatch(string(r.Body))
		if len(avatartok) > 0 {
			avatar = sdk.JoinURL(authConfig.APIURL, avatartok[1])
		}
		acc := sdk.ValidatedAccount{
			ID:          authConfig.APIURL,
			Name:        name,
			Description: authConfig.APIURL,
			AvatarURL:   avatar,
			TotalCount:  resp.Total,
			Type:        "ORG",
			Public:      false,
			Selected:    true,
		}
		return map[string]interface{}{
			"accounts": acc,
		}, nil
	default:
		return nil, fmt.Errorf("unknown action %s", action)
	}
}
