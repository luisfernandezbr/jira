package internal

import (
	"github.com/pinpt/agent.next/sdk"
	"github.com/pinpt/go-common/log"
)

// Export is called to tell the integration to run an export
func (g *JiraIntegration) Export(export sdk.Export) error {
	log.Info(g.logger, "export started")
	return nil
}
