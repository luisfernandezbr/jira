package internal

import (
	"github.com/pinpt/agent/v4/sdk"
)

// AutoConfigure is called when a cloud integration has requested to be auto configured
func (i *JiraIntegration) AutoConfigure(autoconfig sdk.AutoConfigure) (*sdk.Config, error) {
	config := autoconfig.Config()
	// FIXME:
	return &config, nil
}
