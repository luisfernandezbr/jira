<div align="center">
	<img width="500" src=".github/logo.svg" alt="pinpt-logo">
</div>

<p align="center" color="#6a737d">
	<strong>This repo contains the official Jira integration for Pinpoint</strong>
</p>


## Overview

This project contains the source code for the official Jira integration for Pinpoint.

## Features

The following features are supported by this integration:

| Feature             | Export | WebHook | Notes                         |
|---------------------|:------:|:-------:|-------------------------------|
| Cloud               |   ✅   |    ✅   |                              |
| Self Service        |   ✅   |    ✅   |                              |
| Auth: Basic         |   ✅   |    ✅   |                              |
| Auth: API Key       |   🛑   |    🛑   |                              |
| Auth: OAuth2        |   ✅   |    ✅   |                              |
| Repo                |   🛑   |    🛑   |                              |
| Pull Request        |   🛑   |    🛑   |                              |
| Pull Comment        |   🛑   |    🛑   |                              |
| Pull Request Review |   🛑   |    🛑   |                              |
| Project             |   ✅   |    ✅   |                              |
| Epic                |   ✅   |    ✅   |                              |
| Sprint              |   ✅   |    ✅   |                              |
| Kanban              |   ✅   |    ✅   |                              |
| Issue               |   ✅   |    ✅   |                              |
| Issue Comment       |   ✅   |    ✅   |                              |
| Issue Type          |   ✅   |    ✅   |                              |
| Issue Status        |   ✅   |    ✅   |                              |
| Issue Priority      |   ✅   |    ✅   |                              |
| Issue Resolution    |   ✅   |    ✅   |                              |
| Issue Parent/Child  |   ✅   |    ✅   |                              |
| Work Config         |   ✅   |    -    |                              |
| Mutations           |   -    |    ✅   | only with Basic Auth         |
| Feed Notifications  |   🗓   |    🗓   | TODO                         |
| Builds              |   🛑   |    🛑   |                              |
| Deployments         |   🛑   |    🛑   |                              |
| Releases            |   🗓   |    🗓   | TODO                         |
| Security Events     |   🛑   |    🛑   |                              |

## Requirements

You will need the following to build and run locally:

- [Pinpoint Agent SDK](https://github.com/pinpt/agent)
- [Golang](https://golang.org) 1.14+ or later
- [NodeJS](https://nodejs.org) 12+ or later (only if modifying/running the Integration UI)

## Running Locally

You can run locally to test against a repo with the following command (assuming you already have the Agent SDK installed):

```
agent.next dev . --log-level=debug --log-level=debug --channel edge --set "basic_auth={\"url\":\"$PP_JIRA_URL\",\"username\":\"$PP_JIRA_USERNAME\",\"password\":\"$PP_JIRA_PASSWORD\"}"
```

From agent.next repo:

```
go run . -tags dev . dev ../agent.next.jira --log-level=debug --log-level=debug --channel edge --set "basic_auth={\"url\":\"$PP_JIRA_URL\",\"username\":\"$PP_JIRA_USERNAME\",\"password\":\"$PP_JIRA_PASSWORD\"}"
```

Example using an OAuth2 token:

```
go run -tags dev . dev ../agent.next.jira --log-level=debug --channel edge --set "oauth2_auth={\"access_token\":\"TOKEN\",\"refresh_token\":\"REFRESH_TOKEN\"}"
```

Make sure you replace TOKEN and REFRESH_TOKEN.

## Contributions

We ♥️ open source and would love to see your contributions (documentation, questions, pull requests, isssue, etc). Please open an Issue or PullRequest!  If you have any questions or issues, please do not hesitate to let us know.

## License

This code is open source and licensed under the terms of the MIT License. Copyright &copy; 2020 by Pinpoint Software, Inc.
