package internal

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/pinpt/agent.next/sdk"
)

type authConfig struct {
	WebsiteURL string
	APIURL     string
	Middleware []sdk.WithHTTPOption
}

type auth interface {
	Apply() (authConfig, error)
}

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
			func(req *sdk.HTTPRequest) error {
				req.Request.SetBasicAuth(a.username, a.password)
				return nil
			},
		},
	}, nil
}

type oauth2Auth struct {
	accessToken   string
	refreshToken  string
	websiteURL    string
	apiURL        string
	lastRefreshed time.Time
	mu            sync.Mutex
}

var _ auth = (*oauth2Auth)(nil)

func (a *oauth2Auth) refresh() error {
	// TODO:
	return nil
}

func (a *oauth2Auth) Apply() (authConfig, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.lastRefreshed.IsZero() || time.Since(a.lastRefreshed) > time.Hour {
		// FIXME: refresh the token
		if err := a.refresh(); err != nil {
			return authConfig{}, err
		}
	}
	return authConfig{
		WebsiteURL: a.websiteURL,
		APIURL:     a.apiURL,
		Middleware: []sdk.WithHTTPOption{
			sdk.WithAuthorization("Bearer " + a.accessToken),
		},
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
					newToken, err := manager.RefreshOAuth2Token(refType, refreshToken)
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
		if item.URL == url {
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

func newAuth(logger sdk.Logger, manager sdk.Manager, httpmanager sdk.HTTPClientManager, config sdk.Config) (auth, error) {
	ok, url := config.GetString("url")
	if ok {
		theurl, err := fixURLPath(url)
		if err != nil {
			return nil, err
		}
		if ok, accessToken := config.GetString("access_token"); ok {
			ok, refreshToken := config.GetString("refresh_token")
			if !ok {
				return nil, fmt.Errorf("missing required refresh_token config")
			}
			return newOAuth2Auth(logger, manager, httpmanager, theurl, accessToken, refreshToken)
		}
		ok, username := config.GetString("username")
		if ok {
			ok, password := config.GetString("password")
			if ok {
				return &basicAuth{
					url:      theurl,
					username: username,
					password: password,
				}, nil
			}
		}
		return nil, fmt.Errorf("no authentication provided")
	}
	return nil, fmt.Errorf("no url provided")
}
