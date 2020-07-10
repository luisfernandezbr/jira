module github.com/pinpt/agent.next.jira

go 1.14

require (
	github.com/AlecAivazis/survey/v2 v2.0.8 // indirect
	github.com/go-redis/redis/v8 v8.0.0-beta.6 // indirect
	github.com/mailru/easyjson v0.7.1
	github.com/mattn/go-colorable v0.1.7 // indirect
	github.com/mgutz/ansi v0.0.0-20200706080929-d51e80ef957d // indirect
	github.com/pinpt/adf v1.1.0
	github.com/pinpt/agent.next v0.0.0-20200710141020-64549f82cef4
	golang.org/x/net v0.0.0-20200707034311-ab3426394381 // indirect
	golang.org/x/text v0.3.3 // indirect
)

// TODO: this is only set while we're in rapid dev. once we get out of that we should remove
replace github.com/pinpt/agent.next => ../agent.next
