package internal

import "github.com/pinpt/agent.next/sdk"

// Validate TODO
func (i *JiraIntegration) Validate(config sdk.Config) (map[string]interface{}, error) {
	sdk.LogDebug(i.logger, "Validation", "config", config)
	return nil, nil
}
