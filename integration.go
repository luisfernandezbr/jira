package main

import (
	"github.com/pinpt/agent.next.jira/internal"
	"github.com/pinpt/agent.next/runner"

	// so we can dump the stack!
	_ "github.com/songgao/stacktraces/on/SIGUSR2"
)

// Integration is used to export the integration
var Integration internal.JiraIntegration

func main() {
	runner.Main(&Integration)
}
