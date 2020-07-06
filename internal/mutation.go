package internal

import "github.com/pinpt/agent.next/sdk"

type idValue struct {
	ID string `json:"id"`
}

type valueValue struct {
	Value string `json:"value"`
}

type setMutationOperation struct {
	Set interface{} `json:"set"`
}

type mutationRequest struct {
	Update     map[string][]setMutationOperation `json:"update,omitempty"`
	Transition *idValue                          `json:"transition,omitempty"`
	Fields     map[string]interface{}            `json:"fields,omitempty"`
}

func newMutation() mutationRequest {
	return mutationRequest{
		Fields: make(map[string]interface{}),
		Update: make(map[string][]setMutationOperation),
	}
}

// Mutation is called when a mutation request is received on behalf of the integration
func (i *JiraIntegration) Mutation(mutation sdk.Mutation) error {
	logger := sdk.LogWith(i.logger, "customer_id", mutation.CustomerID(), "id", mutation.ID(), "action", mutation.Action(), "model", mutation.Model())
	sdk.LogInfo(logger, "mutation request received")
	user := mutation.User()
	var c sdk.Config // copy in the config for the user
	c.APIKeyAuth = user.APIKeyAuth
	c.BasicAuth = user.BasicAuth
	c.OAuth2Auth = user.OAuth2Auth
	state, err := i.newState(logger, mutation.Pipe(), c, false, mutation.IntegrationInstanceID())
	if err != nil {
		return err
	}
	// TODO:
	// create/update sprint
	// create issue
	switch mutation.Action() {
	case sdk.CreateAction:
		break
	case sdk.UpdateAction:
		switch v := mutation.Payload().(type) {
		case *sdk.WorkIssueUpdateMutation:
			return i.updateIssue(state, mutation, v)
		}
	case sdk.DeleteAction:
		break
	}
	return nil
}
