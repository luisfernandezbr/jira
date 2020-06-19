module github.com/pinpt/agent.next.jira

go 1.14

require (
	github.com/pinpt/adf v1.1.0
	github.com/pinpt/agent.next v0.0.0-20200619184400-94f8ca2838e8
	github.com/pinpt/integration-sdk v0.0.1032 // indirect
	golang.org/x/sys v0.0.0-20200615200032-f1bc736245b1 // indirect
	golang.org/x/text v0.3.3 // indirect
)

// TODO: this is only set while we're in rapid dev. once we get out of that we should remove
replace github.com/pinpt/agent.next => ../agent.next
