module github.com/pinpt/agent.next.jira

go 1.14

require (
	github.com/mailru/easyjson v0.7.1
	github.com/pinpt/adf v1.1.0
	github.com/pinpt/agent.next v0.0.0-20200731184117-1af96b66757e
	github.com/pinpt/integration-sdk v0.0.1223
	github.com/stretchr/testify v1.6.1
	golang.org/x/net v0.0.0-20200707034311-ab3426394381 // indirect
)

replace github.com/pinpt/agent.next => ../agent.next
