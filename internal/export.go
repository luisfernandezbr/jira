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
		sdk.LogInfo(i.logger, "rate limited", "until", time.Now().Add(dur), "customer_id", export.CustomerID())
		time.Sleep(dur)
		sdk.LogInfo(i.logger, "rate limit wake up", "customer_id", export.CustomerID())
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
		sdk.LogInfo(i.logger, "rate limited", "until", resetAt, "customer_id", export.CustomerID())
		time.Sleep(dur)
		sdk.LogInfo(i.logger, "rate limit wake up", "customer_id", export.CustomerID())
		// send a resume now that we're no longer rate limited
		if err := export.Resumed(); err != nil {
			return err
		}
	}
	return rerr
}

func (i *JiraIntegration) fetchPriorities(state *exportState) error {
	theurl := sdk.JoinURL(state.authConfig.APIURL, "/rest/api/3/priority")
	client := i.httpmanager.New(theurl, nil)
	resp := make([]issuePriority, 0)
	ts := time.Now()
	r, err := client.Get(&resp, state.authConfig.Middleware...)
	customerID := state.export.CustomerID()
	for _, p := range resp {
		priority, err := p.ToModel(customerID)
		if err != nil {
			return err
		}
		if err := state.pipe.Write(priority); err != nil {
			return err
		}
		state.stats.incPriority()
	}
	if err := i.checkForRateLimit(state.export, err, r.Headers); err != nil {
		return err
	}
	sdk.LogDebug(state.logger, "fetched priorities", "len", len(resp), "duration", time.Since(ts))
	return nil
}

func (i *JiraIntegration) fetchTypes(state *exportState) error {
	theurl := sdk.JoinURL(state.authConfig.APIURL, "/rest/api/3/issuetype")
	client := i.httpmanager.New(theurl, nil)
	resp := make([]issueType, 0)
	ts := time.Now()
	r, err := client.Get(&resp, state.authConfig.Middleware...)
	customerID := state.export.CustomerID()
	for _, t := range resp {
		issuetype, err := t.ToModel(customerID)
		if err != nil {
			return err
		}
		if err := state.pipe.Write(issuetype); err != nil {
			return err
		}
		state.stats.incType()
	}
	if err := i.checkForRateLimit(state.export, err, r.Headers); err != nil {
		return err
	}
	sdk.LogDebug(state.logger, "fetched issue types", "len", len(resp), "duration", time.Since(ts))
	return nil
}

func (i *JiraIntegration) fetchCustomFields(logger sdk.Logger, export sdk.Export, authConfig authConfig) (map[string]customField, error) {
	theurl := sdk.JoinURL(authConfig.APIURL, "/rest/api/3/field")
	client := i.httpmanager.New(theurl, nil)
	resp := make([]customFieldQueryResult, 0)
	ts := time.Now()
	r, err := client.Get(&resp, authConfig.Middleware...)
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
	sdk.LogDebug(logger, "fetched custom fields", "len", len(resp), "duration", time.Since(ts))
	return customfields, nil
}

func (i *JiraIntegration) fetchProjectsPaginated(state *exportState) error {
	theurl := sdk.JoinURL(state.authConfig.APIURL, "/rest/api/3/project/search")
	client := i.httpmanager.New(theurl, nil)
	queryParams := make(url.Values)
	queryParams.Set("expand", "description,url,issueTypes,projectKeys")
	queryParams.Set("typeKey", "software")
	queryParams.Set("status", "live")
	queryParams.Set("maxResults", "100") // 100 is the max, 50 is the default
	var count int
	customerID := state.export.CustomerID()
	started := time.Now()
	for {
		queryParams.Set("startAt", strconv.Itoa(count))
		var resp projectQueryResult
		ts := time.Now()
		r, err := client.Get(&resp, append(state.authConfig.Middleware, sdk.WithGetQueryParameters(queryParams))...)
		if err := i.checkForRateLimit(state.export, err, r.Headers); err != nil {
			return err
		}
		sdk.LogDebug(state.logger, "fetched projects", "len", len(resp.Projects), "total", resp.Total, "count", count, "first", resp.Projects[0].Key, "last", resp.Projects[len(resp.Projects)-1].Key, "duration", time.Since(ts))
		count += len(resp.Projects)
		for _, p := range resp.Projects {
			project, err := p.ToModel(customerID)
			if err != nil {
				return err
			}
			if err := state.pipe.Write(project); err != nil {
				return err
			}
			state.stats.incProject()
		}
		if count >= resp.Total {
			break
		}
	}
	sdk.LogInfo(state.logger, "export projects completed", "duration", time.Since(started), "count", count)
	return nil
}

func (i *JiraIntegration) fetchIssuesPaginated(state *exportState, fromTime time.Time, customfields map[string]customField) error {
	theurl := sdk.JoinURL(state.authConfig.APIURL, "/rest/api/3/search")
	client := i.httpmanager.New(theurl, nil)
	queryParams := make(url.Values)
	var jql string
	if !fromTime.IsZero() {
		s := relativeDuration(time.Since(fromTime))
		jql = fmt.Sprintf(`(created >= "%s" or updated >= "%s") `, s, s)
	}
	jql += "ORDER BY updated DESC" // search for the most recent changes first
	queryParams.Set("expand", "changelog,fields,comments")
	queryParams.Set("fields", "*navigable,attachment")
	queryParams.Set("jql", jql)
	queryParams.Set("maxResults", "100") // 100 is the max, 50 is the default
	var count int
	customerID := state.export.CustomerID()
	started := time.Now()
	for {
		queryParams.Set("startAt", strconv.Itoa(count))
		var resp issueQueryResult
		ts := time.Now()
		r, err := client.Get(&resp, append(state.authConfig.Middleware, sdk.WithGetQueryParameters(queryParams))...)
		if err := i.checkForRateLimit(state.export, err, r.Headers); err != nil {
			return err
		}
		toprocess := make([]issueSource, 0)
		for _, i := range resp.Issues {
			if !state.issueIDManager.isProcessed(i.Key) {
				state.issueIDManager.cache(i.Key, i.ID) // since we're coming in out of order, try and reduce ref fetches
				state.issueIDManager.cache(i.ID, i.ID)  // do both since you can look it up by either
				toprocess = append(toprocess, i)
			}
		}
		// only process issues that haven't already been processed before (given recursion)
		for _, i := range toprocess {
			issue, comments, err := i.ToModel(customerID, state.issueIDManager, state.sprintManager, state.userManager, customfields, state.authConfig.WebsiteURL)
			if err != nil {
				return err
			}
			if err := state.pipe.Write(issue); err != nil {
				return err
			}
			for _, comment := range comments {
				if err := state.pipe.Write(comment); err != nil {
					return err
				}
				state.stats.incComment()
			}
			state.stats.incIssue()
		}
		if len(resp.Issues) > 0 {
			sdk.LogDebug(state.logger, "fetched issues", "len", len(resp.Issues), "total", resp.Total, "count", count, "first", resp.Issues[0].Key, "last", resp.Issues[len(resp.Issues)-1].Key, "duration", time.Since(ts))
		} else {
			sdk.LogDebug(state.logger, "fetched issues", "len", len(resp.Issues), "total", resp.Total, "count", count, "duration", time.Since(ts))
		}
		count += len(resp.Issues)
		if count >= resp.Total {
			break
		}
	}
	sdk.LogInfo(state.logger, "export issues completed", "duration", time.Since(started), "count", count)
	return nil
}

const configKeyLastExportTimestamp = "last_export_ts"

// Export is called to tell the integration to run an export
func (i *JiraIntegration) Export(export sdk.Export) error {
	logger := sdk.LogWith(i.logger, "customer_id", export.CustomerID(), "job_id", export.JobID())
	sdk.LogInfo(logger, "export started")
	pipe, err := export.Pipe()
	if err != nil {
		return err
	}
	config := export.Config()
	auth, err := newAuth(logger, i.manager, i.httpmanager, config)
	if err != nil {
		return err
	}
	authConfig, err := auth.Apply()
	if err != nil {
		return err
	}
	stats := &stats{
		started: time.Now(),
	}
	var fromTime time.Time
	var fromTimeStr string
	if _, err := export.State().Get(refType, configKeyLastExportTimestamp, &fromTimeStr); err != nil {
		return err
	}
	if fromTimeStr != "" {
		fromTime, _ = time.Parse(time.RFC3339Nano, fromTimeStr)
		sdk.LogInfo(logger, "will start from a specific timestamp", "time", fromTime)
	} else {
		sdk.LogInfo(logger, "no specific timestamp found, will start from now")
	}
	customfields, err := i.fetchCustomFields(logger, export, authConfig)
	sprintManager := newSprintManager(export.CustomerID(), pipe, stats)
	userManager := newUserManager(export.CustomerID(), authConfig.WebsiteURL, pipe, stats)
	issueIDManager := newIssueIDManager(logger, i, export, pipe, sprintManager, userManager, customfields, authConfig, stats)
	exportState := &exportState{
		export:         export,
		pipe:           pipe,
		config:         config,
		authConfig:     authConfig,
		sprintManager:  sprintManager,
		userManager:    userManager,
		issueIDManager: issueIDManager,
		stats:          stats,
		logger:         logger,
	}
	if err := i.fetchProjectsPaginated(exportState); err != nil {
		return err
	}
	if err := i.fetchPriorities(exportState); err != nil {
		return err
	}
	if err := i.fetchTypes(exportState); err != nil {
		return err
	}
	if err := i.fetchIssuesPaginated(exportState, fromTime, customfields); err != nil {
		return err
	}
	if err := pipe.Close(); err != nil {
		return err
	}
	if err := export.State().Set(refType, configKeyLastExportTimestamp, stats.started.Format(time.RFC3339Nano)); err != nil {
		return err
	}
	exportState.stats.dump(logger)
	return nil
}
