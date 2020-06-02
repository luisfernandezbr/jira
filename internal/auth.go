package internal

import (
	"fmt"
	"net/url"

	"github.com/pinpt/agent.next/sdk"
)

type auth interface {
	Apply() (string, []sdk.WithHTTPOption, error)
}

type basicAuth struct {
	url      string
	username string
	password string
}

func (a basicAuth) Apply() (string, []sdk.WithHTTPOption, error) {
	return a.url, []sdk.WithHTTPOption{
		func(req *sdk.HTTPRequest) error {
			req.Request.SetBasicAuth(a.username, a.password)
			return nil
		},
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

func newAuth(config sdk.Config) (auth, error) {
	ok, url := config.GetString("url")
	if ok {
		theurl, err := fixURLPath(url)
		if err != nil {
			return nil, err
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
	}
	return nil, fmt.Errorf("no authentication provided")
}
