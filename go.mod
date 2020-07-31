module github.com/pinpt/agent.next.jira

go 1.14

require (
	github.com/mailru/easyjson v0.7.1
	github.com/pinpt/adf v1.1.0
	github.com/pinpt/agent.next v0.0.0-20200717234012-abc9951e8d51
	github.com/pinpt/integration-sdk v0.0.1117
	github.com/stretchr/testify v1.6.1
	go.opentelemetry.io/otel v0.8.0 // indirect
	golang.org/x/net v0.0.0-20200707034311-ab3426394381 // indirect
	golang.org/x/text v0.3.3 // indirect
)

// TODO: this is only set while we're in rapid dev. once we get out of that we should remove
replace github.com/pinpt/agent.next => ../agent.next
