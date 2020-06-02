package internal

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
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

func (i *JiraIntegration) fetchPriorities(export sdk.Export, baseurl string) error {
	theurl := baseurl + "/rest/api/3/priority"
	client := i.httpmanager.New(theurl, nil)
	resp := make([]priorityQueryResult, 0)
	ts := time.Now()
	r, err := client.Get(&resp, func(req *sdk.HTTPRequest) error {
		// FIXME: remove this
		req.Request.SetBasicAuth(os.Getenv("PP_JIRA_USERNAME"), os.Getenv("PP_JIRA_PASSWORD"))
		return nil
	})
	if err := i.checkForRateLimit(export, err, r.Headers); err != nil {
		return err
	}
	sdk.LogDebug(i.logger, "fetched priorities", "len", len(resp), "duration", time.Since(ts))
	return nil
}

func (i *JiraIntegration) fetchProjectsPaginated(export sdk.Export, baseurl string) error {
	theurl := baseurl + "/rest/api/3/project/search"
	client := i.httpmanager.New(theurl, nil)
	queryParams := make(url.Values)
	queryParams.Set("expand", "description,url,issueTypes,projectKeys")
	queryParams.Set("typeKey", "software")
	queryParams.Set("status", "live")
	queryParams.Set("maxResults", "100") // 100 is the max, 50 is the default
	var count int
	started := time.Now()
	for {
		queryParams.Set("startAt", strconv.Itoa(count))
		var resp projectQueryResult
		ts := time.Now()
		r, err := client.Get(&resp, func(req *sdk.HTTPRequest) error {
			// FIXME: remove this
			req.Request.SetBasicAuth(os.Getenv("PP_JIRA_USERNAME"), os.Getenv("PP_JIRA_PASSWORD"))
			return nil
		}, sdk.WithGetQueryParameters(queryParams))
		if err := i.checkForRateLimit(export, err, r.Headers); err != nil {
			return err
		}
		sdk.LogDebug(i.logger, "fetched projects", "len", len(resp.Projects), "total", resp.Total, "count", count, "first", resp.Projects[0].Key, "last", resp.Projects[len(resp.Projects)-1].Key, "duration", time.Since(ts))
		count += len(resp.Projects)
		if count >= resp.Total {
			break
		}
	}
	sdk.LogInfo(i.logger, "export projects completed", "duration", time.Since(started), "count", count)
	return nil
}

func (i *JiraIntegration) fetchIssuesPaginated(export sdk.Export, baseurl string, fromTime time.Time) error {
	theurl := baseurl + "/rest/api/3/search"
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
	started := time.Now()
	for {
		queryParams.Set("startAt", strconv.Itoa(count))
		var resp issueQueryResult
		ts := time.Now()
		r, err := client.Get(&resp, func(req *sdk.HTTPRequest) error {
			// FIXME: remove this
			req.Request.SetBasicAuth(os.Getenv("PP_JIRA_USERNAME"), os.Getenv("PP_JIRA_PASSWORD"))
			return nil
		}, sdk.WithGetQueryParameters(queryParams))
		if err := i.checkForRateLimit(export, err, r.Headers); err != nil {
			return err
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
	baseurl := "https://pinpt-hq.atlassian.net"
	if err := i.fetchProjectsPaginated(export, baseurl); err != nil {
		return err
	}
	if err := i.fetchPriorities(export, baseurl); err != nil {
		return err
	}
	if err := i.fetchIssuesPaginated(export, baseurl, time.Time{}); err != nil {
		return err
	}
	if err := pipe.Close(); err != nil {
		return err
	}
	export.Completed(nil)
	return nil
}
