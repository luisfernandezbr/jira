module github.com/pinpt/agent.next.jira

go 1.14

require (
	github.com/JohannesKaufmann/html-to-markdown v0.0.0-20200716201554-b46a9fbd5b11 // indirect
	github.com/go-redis/redis/v8 v8.0.0-beta.6 // indirect
	github.com/mailru/easyjson v0.7.1
	github.com/pinpt/adf v1.1.0
	github.com/pinpt/agent.next v0.0.0-20200717211020-a80e2326049f
	go.opentelemetry.io/otel v0.8.0 // indirect
	golang.org/x/net v0.0.0-20200707034311-ab3426394381 // indirect
	golang.org/x/text v0.3.3 // indirect
)

// TODO: this is only set while we're in rapid dev. once we get out of that we should remove
replace github.com/pinpt/agent.next => ../agent.next
