package internal

import (
	"crypto/rsa"

	"github.com/pinpt/agent/sdk"
)

var _ sdk.OAuth1Integration = (*JiraIntegration)(nil)

// IdentifyOAuth1User should be implemented to get an identity for a user tied to the private key
func (i *JiraIntegration) IdentifyOAuth1User(identifier sdk.Identifier, url string, privateKey *rsa.PrivateKey, consumerKey string, consumerSecret string, token string, tokenSecret string) (*sdk.OAuth1Identity, error) {
	theurl, err := fixURLPath(url)
	if err != nil {
		return nil, err
	}
	auth := &oauth1Auth{
		apiURL:      theurl,
		token:       token,
		tokenSecret: tokenSecret,
		consumerKey: consumerKey,
		manager:     i.manager,
		identifier:  identifier,
	}
	authConfig, err := auth.Apply()
	if err != nil {
		return nil, err
	}
	client := i.httpmanager.New(sdk.JoinURL(theurl, "/rest/api/3/myself"), nil)
	var resp user
	if _, err := client.Get(&resp, authConfig.Middleware...); err != nil {
		return nil, err
	}
	return &sdk.OAuth1Identity{
		Name:      resp.DisplayName,
		AvatarURL: sdk.StringPointer(resp.Avatars.Large),
		RefID:     resp.RefID(),
	}, nil
}
