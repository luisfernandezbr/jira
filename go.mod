module github.com/pinpt/agent.next.jira

go 1.14

require (
	github.com/dgryski/go-rendezvous v0.0.0-20200624174652-8d2f3be8b2d9 // indirect
	github.com/mattn/go-colorable v0.1.7 // indirect
	github.com/pinpt/adf v1.1.0
	github.com/pinpt/agent.next v0.0.0-20200630234250-ca271ef92d57
	github.com/pinpt/httpclient v0.0.0-20200627153820-d374c2f15648 // indirect
	go.opentelemetry.io/otel v0.7.0 // indirect
	golang.org/x/text v0.3.3 // indirect
)

// TODO: this is only set while we're in rapid dev. once we get out of that we should remove
replace github.com/pinpt/agent.next => ../agent.next
