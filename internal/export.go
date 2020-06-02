package internal

import (
	"github.com/pinpt/agent.next/sdk"
)

// Export is called to tell the integration to run an export
func (g *JiraIntegration) Export(export sdk.Export) error {
	sdk.LogInfo(g.logger, "export started")
	return nil
}
