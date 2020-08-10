package internal

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/pinpt/agent.next/sdk"
)

// easyjson:skip
type authConfig struct {
	WebsiteURL       string
	APIURL           string
	Middleware       []sdk.WithHTTPOption
	SupportsAgileAPI bool
}

type auth interface {
	Apply() (authConfig, error)
}

// easyjson:skip
type basicAuth struct {
	url      string
	username string
	password string
}

var _ auth = (*basicAuth)(nil)

func (a basicAuth) Apply() (authConfig, error) {
	return authConfig{
		WebsiteURL: a.url,
		APIURL:     a.url,
		Middleware: []sdk.WithHTTPOption{
			sdk.WithBasicAuth(a.username, a.password),
		},
		SupportsAgileAPI: true,
	}, nil
}

// easyjson:skip
type oauth2Auth struct {
	accessToken  string
	refreshToken string
	websiteURL   string
	apiURL       string
	manager      sdk.Manager
}

var _ auth = (*oauth2Auth)(nil)

func (a *oauth2Auth) Apply() (authConfig, error) {
	return authConfig{
		WebsiteURL: a.websiteURL,
		APIURL:     a.apiURL,
		Middleware: []sdk.WithHTTPOption{
			sdk.WithOAuth2Refresh(a.manager, refType, a.accessToken, a.refreshToken),
		},
		SupportsAgileAPI: false,
	}, nil
}

// easyjson:skip
type oauth1Auth struct {
	token       string
	tokenSecret string
	consumerKey string
	apiURL      string
	identifier  sdk.Identifier
	manager     sdk.Manager
}

var _ auth = (*oauth1Auth)(nil)

func (a *oauth1Auth) Apply() (authConfig, error) {
	return authConfig{
		WebsiteURL: a.apiURL,
		APIURL:     a.apiURL,
		Middleware: []sdk.WithHTTPOption{
			sdk.WithOAuth1(a.manager, a.identifier, a.consumerKey, reverseString(a.consumerKey), a.token, a.tokenSecret),
		},
		SupportsAgileAPI: true,
	}, nil
}

func newOAuth2Auth(logger sdk.Logger, manager sdk.Manager, httpmanager sdk.HTTPClientManager, url string, accessToken string, refreshToken string) (*oauth2Auth, error) {
	var sites []struct {
		ID  string
		URL string
	}
	var attempts int
	for {
		client := httpmanager.New("https://api.atlassian.com/oauth/token/accessible-resources", map[string]string{
			"Authorization": "Bearer " + accessToken,
			"Content-Type":  "application/json",
		})
		resp, err := client.Get(&sites)
		if err != nil {
			if resp != nil && resp.StatusCode == http.StatusUnauthorized {
				sdk.LogDebug(logger, "oauth2 token accessible-resource failed", "err", err, "attempts", attempts, "status", resp.StatusCode)
				if attempts == 0 {
					attempts++
					newToken, err := manager.AuthManager().RefreshOAuth2Token(refType, refreshToken)
					if err == nil {
						accessToken = newToken
						continue
					}
					sdk.LogDebug(logger, "oauth2 refresh token failed", "err", err)
				}
				return nil, errors.New("Auth token provided is not correct. Getting status 401 (Unauthorized) when trying to call api.atlassian.com/oauth/token/accessible-resources")
			}
			return nil, err
		}
		break
	}
	if len(sites) == 0 {
		return nil, errors.New("no accessible-resources resources found for oauth token")
	}
	var siteid, siteurl string
	for _, item := range sites {
		if item.URL == url || url == "" {
			siteid = item.ID
			siteurl = item.URL
			break
		}
	}
	if siteurl == "" {
		var authed []string
		for _, item := range sites {
			authed = append(authed, item.URL)
		}
		return nil, fmt.Errorf("This account is not authorized for Jira with the following url: %v, it can only access the following instances: %v", url, authed)
	}
	return &oauth2Auth{
		websiteURL:   url,
		apiURL:       "https://api.atlassian.com/ex/jira/" + siteid,
		accessToken:  accessToken,
		refreshToken: refreshToken,
		manager:      manager,
	}, nil
}

func fixURLPath(theurl string) (string, error) {
	u, err := url.Parse(theurl)
	if err != nil {
		return "", err
	}
	u.Path = ""
	return u.String(), nil
}

func newAuth(logger sdk.Logger, manager sdk.Manager, identifier sdk.Identifier, httpmanager sdk.HTTPClientManager, config sdk.Config) (auth, error) {
	if config.OAuth1Auth != nil {
		theurl, err := fixURLPath(config.OAuth1Auth.URL)
		if err != nil {
			return nil, err
		}
		sdk.LogInfo(logger, "using oauth1 authentication")
		return &oauth1Auth{
			apiURL:      theurl,
			token:       config.OAuth1Auth.Token,
			tokenSecret: config.OAuth1Auth.Secret,
			consumerKey: config.OAuth1Auth.ConsumerKey,
			manager:     manager,
			identifier:  identifier,
		}, nil
	}
	if config.OAuth2Auth != nil {
		var refreshToken string
		if config.OAuth2Auth.RefreshToken != nil {
			refreshToken = *config.OAuth2Auth.RefreshToken
		}
		theurl, err := fixURLPath(config.OAuth2Auth.URL)
		if err != nil {
			return nil, err
		}
		sdk.LogInfo(logger, "using oauth2 authentication")
		return newOAuth2Auth(logger, manager, httpmanager, theurl, config.OAuth2Auth.AccessToken, refreshToken)
	}
	if config.BasicAuth != nil {
		theurl, err := fixURLPath(config.BasicAuth.URL)
		if err != nil {
			return nil, err
		}
		sdk.LogInfo(logger, "using basic authentication")
		return &basicAuth{
			url:      theurl,
			username: config.BasicAuth.Username,
			password: config.BasicAuth.Password,
		}, nil
	}
	return nil, fmt.Errorf("authentication provided is not supported. tried oauth2 and basic authentication")
}
