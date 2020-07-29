package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/pinpt/adf"
	"github.com/pinpt/agent.next/sdk"
)

type comment struct {
	Self    string          `json:"self"`
	ID      string          `json:"id"`
	Author  user            `json:"author"`
	Body    json.RawMessage `json:"body"`
	Created string          `json:"created"`
	Updated string          `json:"updated"`

	/**
		there is a visibility flag so we probably at some point want to consider
		bringing that into the model
		"visibility": {
	        "type": "role",
	        "value": "Administrators"
	      }*/
}

func (c comment) ToModel(customerID string, integrationInstanceID string, websiteURL string, userManager UserManager, projectID string, issueID string, issueKey string) (*sdk.WorkIssueComment, error) {
	if err := userManager.Emit(c.Author); err != nil {
		return nil, err
	}
	comment := &sdk.WorkIssueComment{}
	comment.Active = true
	comment.CustomerID = customerID
	comment.IntegrationInstanceID = sdk.StringPointer(integrationInstanceID)
	comment.RefID = c.ID
	comment.RefType = refType
	comment.ProjectID = projectID
	comment.IssueID = issueID
	created, err := parseTime(c.Created)
	if err != nil {
		return nil, err
	}
	sdk.ConvertTimeToDateModel(created, &comment.CreatedDate)
	updated, err := parseTime(c.Updated)
	if err != nil {
		return nil, err
	}
	sdk.ConvertTimeToDateModel(updated, &comment.UpdatedDate)
	comment.UserRefID = c.Author.RefID()
	comment.URL = issueCommentURL(websiteURL, issueKey, c.ID)

	if c.Body != nil {
		html, err := adf.GenerateHTMLFromADF(c.Body)
		if err != nil {
			return nil, fmt.Errorf("error parsing comment body: %w", err)
		}
		comment.Body = adjustRenderedHTML(websiteURL, html)
	}
	return comment, nil
}

func (i *JiraIntegration) fetchComment(authCfg authConfig, userManager UserManager, integrationInstanceID, customerID, issueRefID, issueKey, commentRefID, projectID string) (*sdk.WorkIssueComment, error) {
	theurl := sdk.JoinURL(authCfg.APIURL, fmt.Sprintf("/rest/api/3/issue/%s/comment/%s", issueRefID, commentRefID))
	client := i.httpmanager.New(theurl, nil)
	issueID := sdk.NewWorkIssueID(customerID, issueRefID, refType)
	qs := url.Values{}
	var c comment
	resp, err := client.Get(&c, append(authCfg.Middleware, sdk.WithGetQueryParameters(qs))...)
	if resp == nil && err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	return c.ToModel(customerID, integrationInstanceID, authCfg.WebsiteURL, userManager, projectID, issueID, issueKey)
}
