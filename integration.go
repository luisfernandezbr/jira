package main

import (
	"github.com/pinpt/agent.next.jira/internal"
	"github.com/pinpt/agent.next/runner"
)

// Integration is used to export the integration
var Integration internal.JiraIntegration

func main() {
	runner.Main(&Integration)
}
