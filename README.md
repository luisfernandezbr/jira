<div align="center">
	<img width="500" src=".github/logo.svg" alt="pinpt-logo">
</div>

<p align="center" color="#6a737d">
	<strong>This repo contains a working prototype for the next gen agent Jira integration</strong>
</p>


## Overview

This is a working concept prototype for the next generation of the Agent's Jira integration.  It's meant to experiment with some different design choices and to validate some potential architectural decisions.

## Running

You can run like this:

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


This will run an export for Jira and print all the JSON objects to the console.
