module github.com/pinpt/agent.next.jira

go 1.14

require (
	github.com/go-redis/redis/v8 v8.0.0-beta.6 // indirect
	github.com/mailru/easyjson v0.7.1
	github.com/mattn/go-colorable v0.1.7 // indirect
	github.com/pinpt/adf v1.1.0
	github.com/pinpt/agent.next v0.0.0-20200707023048-3110ab2ea254
	golang.org/x/text v0.3.3 // indirect
)

// TODO: this is only set while we're in rapid dev. once we get out of that we should remove
replace github.com/pinpt/agent.next => ../agent.next
