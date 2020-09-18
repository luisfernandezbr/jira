package internal

import (
	"fmt"
	"time"

	"github.com/pinpt/agent.next/sdk"
)

type status struct {
	Name           string         `json:"name"`
	StatusCategory statusCategory `json:"statusCategory"`
	ID             string         `json:"id"`
	Description    string         `json:"description"`
	IconURL        string         `json:"iconUrl"`
}

const cacheKeyWorkConfig = "work_config"

func (i *JiraIntegration) processWorkConfig(config sdk.Config, pipe sdk.Pipe, istate sdk.State, customerID string, integrationInstanceID string, historical bool) error {
	logger := sdk.LogWith(i.logger, "customer_id", customerID, "integration_instance_id", integrationInstanceID)
	sdk.LogInfo(logger, "processing work config started")
	authConfig, err := i.createAuthConfigFromConfig(sdk.NewSimpleIdentifier(customerID, integrationInstanceID, refType), config)
	if err != nil {
		return fmt.Errorf("error creating work config: %w", err)
	}
	// FIXME(robin): why does a new state need to be declared here instead of useing the one passed in?
	state := i.newState(logger, pipe, authConfig, config, historical, integrationInstanceID)
	theurl := sdk.JoinURL(state.authConfig.APIURL, "/rest/api/3/status")
	client := i.httpmanager.New(theurl, nil)
	resp := make([]status, 0)
	ts := time.Now()
	r, err := client.Get(&resp, state.authConfig.Middleware...)
	if err := i.checkForRateLimit(state.export, customerID, err, r.Headers); err != nil {
		return err
	}
	var wc sdk.WorkConfig
	wc.ID = sdk.NewWorkConfigID(customerID, refType, integrationInstanceID)
	wc.IntegrationInstanceID = integrationInstanceID
	wc.CustomerID = customerID
	wc.RefType = refType
	wc.Statuses = sdk.WorkConfigStatuses{
		OpenStatus:       make([]string, 0),
		InProgressStatus: make([]string, 0),
		ClosedStatus:     make([]string, 0),
	}
	found := make(map[string]bool)
	for _, status := range resp {
		if !found[status.Name] {
			found[status.Name] = true
			switch status.StatusCategory.Key {
			case statusCategoryNew:
				wc.Statuses.OpenStatus = append(wc.Statuses.OpenStatus, status.Name)
			case statusCategoryDone:
				wc.Statuses.ClosedStatus = append(wc.Statuses.ClosedStatus, status.Name)
			case statusCategoryIntermediate:
				wc.Statuses.InProgressStatus = append(wc.Statuses.InProgressStatus, status.Name)
			}
		}
	}
	var existingWorkConfigHashCode string
	// only send any changes either (a) not in cache OR (b) the hash codes are different indicating a change
	if found, _ := istate.Get(cacheKeyWorkConfig, &existingWorkConfigHashCode); historical || !found || existingWorkConfigHashCode != wc.Hash() {
		if !found {
			wc.CreatedAt = sdk.EpochNow()
		}
		wc.UpdatedAt = sdk.EpochNow()
		if err := pipe.Write(&wc); err != nil {
			return fmt.Errorf("error writing work status config to pipe: %w", err)
		}
		if err := istate.Set(cacheKeyWorkConfig, wc.Hash()); err != nil {
			return fmt.Errorf("error writing work status config key to cache: %w", err)
		}
		sdk.LogDebug(state.logger, "processed work config", "len", len(resp), "duration", time.Since(ts))
		return nil
	}
	sdk.LogDebug(state.logger, "processed work config (no changes)", "len", len(resp), "duration", time.Since(ts))
	return nil
}
