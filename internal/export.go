package internal

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/pinpt/agent.next/sdk"
)

func (i *JiraIntegration) checkForRateLimit(export sdk.Export, rerr error, header http.Header) error {
	if ok, dur := sdk.IsRateLimitError(rerr); ok {
		// pause until we are no longer rate limited
		sdk.LogInfo(i.logger, "rate limited", "until", time.Now().Add(dur))
		time.Sleep(dur)
		sdk.LogInfo(i.logger, "rate limit wake up")
		// send a resume now that we're no longer rate limited
		if err := export.Resumed(); err != nil {
			return err
		}
		return nil
	}
	// check for rate limit headers
	limit := header.Get("X-RateLimit-Limit")
	total := header.Get("X-RateLimit-Remaining")
	var shouldPause bool
	if limit != "" && total != "" {
		l, _ := strconv.ParseInt(limit, 10, 32)
		t, _ := strconv.ParseInt(total, 10, 32)
		shouldPause = float32(t)*.8 >= float32(l)
	}
	if shouldPause {
		dur := time.Minute * 5
		resetAt := time.Now().Add(dur)
		if err := export.Paused(resetAt); err != nil {
			return err
		}
		// pause until we are no longer rate limited
		sdk.LogInfo(i.logger, "rate limited", "until", resetAt)
		time.Sleep(dur)
		sdk.LogInfo(i.logger, "rate limit wake up")
		// send a resume now that we're no longer rate limited
		if err := export.Resumed(); err != nil {
			return err
		}
	}
	return rerr
}

func (i *JiraIntegration) fetchPriorities(export sdk.Export, pipe sdk.Pipe, auth auth) error {
	baseurl, middleware, err := auth.Apply()
	if err != nil {
		return err
	}
	theurl := sdk.JoinURL(baseurl, "/rest/api/3/priority")
	client := i.httpmanager.New(theurl, nil)
	resp := make([]issuePriority, 0)
	ts := time.Now()
	r, err := client.Get(&resp, middleware...)
	customerID := export.CustomerID()
	for _, p := range resp {
		priority, err := p.ToModel(customerID)
		if err != nil {
			return err
		}
		if err := pipe.Write(priority); err != nil {
			return err
		}
	}
	if err := i.checkForRateLimit(export, err, r.Headers); err != nil {
		return err
	}
	sdk.LogDebug(i.logger, "fetched priorities", "len", len(resp), "duration", time.Since(ts))
	return nil
}

func (i *JiraIntegration) fetchTypes(export sdk.Export, pipe sdk.Pipe, auth auth) error {
	baseurl, middleware, err := auth.Apply()
	if err != nil {
		return err
	}
	theurl := sdk.JoinURL(baseurl, "/rest/api/3/issuetype")
	client := i.httpmanager.New(theurl, nil)
	resp := make([]issueType, 0)
	ts := time.Now()
	r, err := client.Get(&resp, middleware...)
	customerID := export.CustomerID()
	for _, t := range resp {
		issuetype, err := t.ToModel(customerID)
		if err != nil {
			return err
		}
		if err := pipe.Write(issuetype); err != nil {
			return err
		}
	}
	if err := i.checkForRateLimit(export, err, r.Headers); err != nil {
		return err
	}
	sdk.LogDebug(i.logger, "fetched issue types", "len", len(resp), "duration", time.Since(ts))
	return nil
}

func (i *JiraIntegration) fetchCustomFields(export sdk.Export, auth auth) (map[string]customField, error) {
	baseurl, middleware, err := auth.Apply()
	if err != nil {
		return nil, err
	}
	theurl := sdk.JoinURL(baseurl, "/rest/api/3/field")
	client := i.httpmanager.New(theurl, nil)
	resp := make([]customFieldQueryResult, 0)
	ts := time.Now()
	r, err := client.Get(&resp, middleware...)
	if err := i.checkForRateLimit(export, err, r.Headers); err != nil {
		return nil, err
	}
	customfields := map[string]customField{}
	for _, r := range resp {
		var field customField
		if r.Key != "" {
			field.ID = r.Key
		} else {
			field.ID = r.ID
		}
		field.Name = r.Name
		customfields[field.ID] = field
	}
	sdk.LogDebug(i.logger, "fetched custom fields", "len", len(resp), "duration", time.Since(ts))
	return customfields, nil
}

func (i *JiraIntegration) fetchProjectsPaginated(export sdk.Export, pipe sdk.Pipe, auth auth) error {
	baseurl, middleware, err := auth.Apply()
	if err != nil {
		return err
	}
	theurl := sdk.JoinURL(baseurl, "/rest/api/3/project/search")
	client := i.httpmanager.New(theurl, nil)
	queryParams := make(url.Values)
	queryParams.Set("expand", "description,url,issueTypes,projectKeys")
	queryParams.Set("typeKey", "software")
	queryParams.Set("status", "live")
	queryParams.Set("maxResults", "100") // 100 is the max, 50 is the default
	var count int
	customerID := export.CustomerID()
	started := time.Now()
	for {
		queryParams.Set("startAt", strconv.Itoa(count))
		var resp projectQueryResult
		ts := time.Now()
		r, err := client.Get(&resp, append(middleware, sdk.WithGetQueryParameters(queryParams))...)
		if err := i.checkForRateLimit(export, err, r.Headers); err != nil {
			return err
		}
		sdk.LogDebug(i.logger, "fetched projects", "len", len(resp.Projects), "total", resp.Total, "count", count, "first", resp.Projects[0].Key, "last", resp.Projects[len(resp.Projects)-1].Key, "duration", time.Since(ts))
		count += len(resp.Projects)
		for _, p := range resp.Projects {
			project, err := p.ToModel(customerID)
			if err != nil {
				return err
			}
			if err := pipe.Write(project); err != nil {
				return err
			}
		}
		if count >= resp.Total {
			break
		}
	}
	sdk.LogInfo(i.logger, "export projects completed", "duration", time.Since(started), "count", count)
	return nil
}

func (i *JiraIntegration) fetchIssuesPaginated(export sdk.Export, pipe sdk.Pipe, issueManager *issueIDManager, sprintManager *sprintManager, userManager *userManager, auth auth, fromTime time.Time, customfields map[string]customField) error {
	baseurl, middleware, err := auth.Apply()
	if err != nil {
		return err
	}
	theurl := sdk.JoinURL(baseurl, "/rest/api/3/search")
	client := i.httpmanager.New(theurl, nil)
	queryParams := make(url.Values)
	var jql string
	if !fromTime.IsZero() {
		s := relativeDuration(time.Since(fromTime))
		jql = fmt.Sprintf(`(created >= "%s" or updated >= "%s") `, s, s)
	}
	jql += "ORDER BY updated DESC" // search for the most recent changes first
	queryParams.Set("expand", "changelog,fields,renderedFields")
	queryParams.Set("fields", "*navigable,attachment")
	queryParams.Set("jql", jql)
	queryParams.Set("maxResults", "100") // 100 is the max, 50 is the default
	var count int
	customerID := export.CustomerID()
	started := time.Now()
	for {
		queryParams.Set("startAt", strconv.Itoa(count))
		var resp issueQueryResult
		ts := time.Now()
		r, err := client.Get(&resp, append(middleware, sdk.WithGetQueryParameters(queryParams))...)
		if err := i.checkForRateLimit(export, err, r.Headers); err != nil {
			return err
		}
		toprocess := make([]issueSource, 0)
		for _, i := range resp.Issues {
			if !issueManager.isProcessed(i.Key) {
				issueManager.cache(i.Key, i.ID) // since we're coming in out of order, try and reduce ref fetches
				issueManager.cache(i.ID, i.ID)  // do both since you can look it up by either
				toprocess = append(toprocess, i)
			}
		}
		// only process issues that haven't already been processed before (given recursion)
		for _, i := range toprocess {
			issue, err := i.ToModel(customerID, issueManager, sprintManager, userManager, customfields, baseurl)
			if err != nil {
				return err
			}
			// TODO: changelog
			if err := pipe.Write(issue); err != nil {
				return err
			}
		}
		sdk.LogDebug(i.logger, "fetched issues", "len", len(resp.Issues), "total", resp.Total, "count", count, "first", resp.Issues[0].Key, "last", resp.Issues[len(resp.Issues)-1].Key, "duration", time.Since(ts))
		count += len(resp.Issues)
		if count >= resp.Total {
			break
		}
	}
	sdk.LogInfo(i.logger, "export issues completed", "duration", time.Since(started), "count", count)
	return nil
}

// Export is called to tell the integration to run an export
func (i *JiraIntegration) Export(export sdk.Export) error {
	sdk.LogInfo(i.logger, "export started")
	pipe, err := export.Start()
	if err != nil {
		return err
	}
	config := export.Config()
	auth, err := newAuth(config)
	if err != nil {
		return err
	}
	baseurl, _, err := auth.Apply()
	if err := i.fetchProjectsPaginated(export, pipe, auth); err != nil {
		return err
	}
	if err := i.fetchPriorities(export, pipe, auth); err != nil {
		return err
	}
	if err := i.fetchTypes(export, pipe, auth); err != nil {
		return err
	}
	customfields, err := i.fetchCustomFields(export, auth)
	if err != nil {
		return err
	}
	sprintManager := newSprintManager(export.CustomerID(), pipe)
	userManager := newUserManager(export.CustomerID(), baseurl, pipe)
	issueManager := newIssueIDManager(i, export, pipe, sprintManager, userManager, customfields, baseurl)
	if err := i.fetchIssuesPaginated(export, pipe, issueManager, sprintManager, userManager, auth, time.Time{}, customfields); err != nil {
		return err
	}
	if err := pipe.Close(); err != nil {
		return err
	}
	export.Completed(nil)
	return nil
}
