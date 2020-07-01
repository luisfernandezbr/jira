package internal

import (
	"encoding/json"
	"fmt"

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

func (c comment) ToModel(customerID string, integrationInstanceID string, websiteURL string, userManager *userManager, projectID string, issueID string, issueKey string) (*sdk.WorkIssueComment, error) {
	if err := userManager.emit(c.Author); err != nil {
		return nil, err
	}
	comment := &sdk.WorkIssueComment{}
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
