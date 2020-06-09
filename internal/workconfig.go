package internal

import (
	"time"

	"github.com/pinpt/agent.next/sdk"
)

type status struct {
	Name           string `json:"name"`
	StatusCategory struct {
		Key  string `json:"key"`
		Name string `json:"name"`
	} `json:"statusCategory"`
	ID          string `json:"id"`
	Description string `json:"description"`
	IconURL     string `json:"iconUrl"`
}

func (i *JiraIntegration) fetchStatuses(instance sdk.Instance) error {
	logger := sdk.LogWith(i.logger, "customer_id", instance.CustomerID())
	sdk.LogInfo(logger, "export started")
	state, err := i.newState(logger, instance.Pipe(), instance.Config())
	if err != nil {
		return err
	}
	theurl := sdk.JoinURL(state.authConfig.APIURL, "/rest/api/3/status")
	client := i.httpmanager.New(theurl, nil)
	resp := make([]status, 0)
	ts := time.Now()
	r, err := client.Get(&resp, state.authConfig.Middleware...)
	// FIXME: we need to handle the work status back to the pipe
	if err := i.checkForRateLimit(state.export, err, r.Headers); err != nil {
		return err
	}
	sdk.LogDebug(state.logger, "fetched statuses", "len", len(resp), "duration", time.Since(ts))
	return nil
}
