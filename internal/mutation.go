package internal

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/pinpt/agent/v4/sdk"
)

type idValue struct {
	ID string `json:"id"`
}

type userValue struct {
	AccountID *string `json:"accountId"`
}

type valueValue struct {
	Value string `json:"value"`
}

type keyValue struct {
	Key string `json:"key"`
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
func (i *JiraIntegration) Mutation(mutation sdk.Mutation) (*sdk.MutationResponse, error) {
	logger := sdk.LogWith(i.logger, "customer_id", mutation.CustomerID(), "id", mutation.ID(), "action", mutation.Action(), "model", mutation.Model())
	sdk.LogInfo(logger, "mutation request received")
	user := mutation.User()
	var c sdk.Config // copy in the config for the user
	c.APIKeyAuth = user.APIKeyAuth
	c.BasicAuth = user.BasicAuth
	c.OAuth2Auth = user.OAuth2Auth
	c.OAuth1Auth = user.OAuth1Auth
	authConfig, err := i.createAuthConfigFromConfig(mutation, c)
	if err != nil {
		return nil, fmt.Errorf("error creating auth config: %w", err)
	}
	switch v := mutation.Payload().(type) {
	// Issue
	case *sdk.WorkIssueUpdateMutation:
		return nil, i.updateIssue(logger, mutation, authConfig, v)
	case *sdk.WorkIssueCreateMutation:
		return nil, i.createIssue(logger, mutation, authConfig, v)

	// Sprint
	case *sdk.AgileSprintUpdateMutation:
		if !authConfig.SupportsAgileAPI {
			return nil, errors.New("current authentication does not support agile api")
		}
		return nil, i.updateSprint(logger, mutation, authConfig, v)
	case *sdk.AgileSprintCreateMutation:
		if !authConfig.SupportsAgileAPI {
			return nil, errors.New("current authentication does not support agile api")
		}
		return nil, i.createSprint(logger, mutation, authConfig, v)
	}
	sdk.LogInfo(logger, "unhandled mutation request", "type", reflect.TypeOf(mutation.Payload()))
	return nil, nil
}
