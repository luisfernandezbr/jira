module github.com/pinpt/agent.next.jira

go 1.14

require (
	github.com/pinpt/adf v1.1.0
	github.com/pinpt/agent.next v0.0.0-20200609021955-b4b6fcc78897
)

// TODO: this is only set while we're in rapid dev. once we get out of that we should remove
replace github.com/pinpt/agent.next => ../agent.next
