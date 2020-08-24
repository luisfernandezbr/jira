package internal

import (
	"github.com/pinpt/agent.next/sdk"
)

// AutoConfigure is called when a cloud integration has requested to be auto configured
func (i *JiraIntegration) AutoConfigure(autoconfig sdk.AutoConfigure) (*sdk.Config, error) {
	config := autoconfig.Config()
	// FIXME:
	return &config, nil
}
