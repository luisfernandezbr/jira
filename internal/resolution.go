package internal

import (
	"fmt"

	"github.com/pinpt/agent/v4/sdk"
)

func (i *JiraIntegration) fetchIssueResolutions(state *state) ([]sdk.WorkProjectIssueResolutions, error) {
	theurl := sdk.JoinURL(state.authConfig.APIURL, "/rest/api/3/resolution")
	client := i.httpmanager.New(theurl, nil)
	var resp []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if _, err := client.Get(&resp, state.authConfig.Middleware...); err != nil {
		return nil, fmt.Errorf("error fetching issue type schemes mapping: %w", err)
	}
	results := make([]sdk.WorkProjectIssueResolutions, 0)
	for _, r := range resp {
		results = append(results, sdk.WorkProjectIssueResolutions{
			RefID: r.ID,
			Name:  r.Name,
		})
	}
	return results, nil
}
