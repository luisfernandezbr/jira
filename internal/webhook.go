package internal

import "github.com/pinpt/agent.next/sdk"

// WebHook is called when a webhook is received on behalf of the integration
func (i *JiraIntegration) WebHook(webhook sdk.WebHook) error {
	sdk.LogInfo(i.logger, "webhook request received", "customer_id", webhook.CustomerID())
	return nil
}
