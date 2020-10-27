package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pinpt/agent/v4/sdk"
)

func (i *JiraIntegration) checkForRateLimit(logger sdk.Logger, control sdk.Control, customerID string, rerr error, header http.Header) error {
	if ok, dur := sdk.IsRateLimitError(rerr); ok {
		// pause until we are no longer rate limited
		sdk.LogInfo(logger, "rate limited", "until", time.Now().Add(dur), "customer_id", customerID)
		time.Sleep(dur)
		sdk.LogInfo(logger, "rate limit wake up", "customer_id", customerID)
		// send a resume now that we're no longer rate limited
		if err := control.Resumed(); err != nil {
			return err
		}
		return nil
	}

	//TODO: check this against the new HTTP changes

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
		if err := control.Paused(resetAt); err != nil {
			return err
		}
		// pause until we are no longer rate limited
		sdk.LogInfo(logger, "rate limited", "until", resetAt, "customer_id", customerID)
		time.Sleep(dur)
		sdk.LogInfo(logger, "rate limit wake up", "customer_id", customerID)
		// send a resume now that we're no longer rate limited
		if err := control.Resumed(); err != nil {
			return err
		}
	}
	return rerr
}

func (i *JiraIntegration) fetchPriorities(state *state) error {
	theurl := sdk.JoinURL(state.authConfig.APIURL, "/rest/api/3/priority")
	client := i.httpmanager.New(theurl, nil)
	resp := make([]issuePriority, 0)
	ts := time.Now()
	r, err := client.Get(&resp, state.authConfig.Middleware...)
	customerID := state.export.CustomerID()
	for _, p := range resp {
		priority, err := p.ToModel(customerID, state.integrationInstanceID)
		if err != nil {
			return err
		}
		if err := state.pipe.Write(priority); err != nil {
			return err
		}
		state.stats.incPriority()
	}
	if err := i.checkForRateLimit(state.logger, state.export, customerID, err, r.Headers); err != nil {
		return err
	}
	sdk.LogDebug(state.logger, "fetched priorities", "len", len(resp), "duration", time.Since(ts))
	return nil
}

func (i *JiraIntegration) fetchTypes(state *state) error {
	theurl := sdk.JoinURL(state.authConfig.APIURL, "/rest/api/3/issuetype")
	client := i.httpmanager.New(theurl, nil)
	resp := make([]issueType, 0)
	ts := time.Now()
	r, err := client.Get(&resp, state.authConfig.Middleware...)
	customerID := state.export.CustomerID()
	for _, t := range resp {
		issuetype, err := t.ToModel(customerID, state.integrationInstanceID)
		if err != nil {
			return err
		}
		if err := state.pipe.Write(issuetype); err != nil {
			return err
		}
		state.stats.incType()
	}
	if err := i.checkForRateLimit(state.logger, state.export, customerID, err, r.Headers); err != nil {
		return err
	}
	sdk.LogDebug(state.logger, "fetched issue types", "len", len(resp), "duration", time.Since(ts))
	return nil
}

func (i *JiraIntegration) fetchCustomFields(logger sdk.Logger, control sdk.Control, customerID string, authConfig authConfig) (map[string]customField, error) {
	theurl := sdk.JoinURL(authConfig.APIURL, "/rest/api/3/field")
	client := i.httpmanager.New(theurl, nil)
	resp := make([]customFieldQueryResult, 0)
	ts := time.Now()
	r, err := client.Get(&resp, authConfig.Middleware...)
	if err != nil {
		return nil, err
	}
	if err := i.checkForRateLimit(logger, control, customerID, err, r.Headers); err != nil {
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

func (i *JiraIntegration) fetchIssueCreateMeta(state *state, projectIDs []string) ([]projectIssueCreateMeta, error) {
	theurl := sdk.JoinURL(state.authConfig.APIURL, "/rest/api/3/issue/createmeta")
	client := i.httpmanager.New(theurl, nil)
	queryParams := make(url.Values)
	if len(projectIDs) > 0 {
		queryParams.Set("projectIds", strings.Join(projectIDs, ","))
	}
	queryParams.Set("expand", "projects.issuetypes.fields")
	var resp issueCreateMeta
	r, err := client.Get(&resp, append(state.authConfig.Middleware, sdk.WithGetQueryParameters(queryParams))...)
	if err := i.checkForRateLimit(state.logger, state.export, state.export.CustomerID(), err, r.Headers); err != nil {
		return nil, err
	}
	return resp.Projects, nil
}

const savedPreviousProjectsStateKey = "previous_projects"

func (i *JiraIntegration) fetchProjectsPaginated(state *state) ([]string, error) {
	resolutions, err := i.fetchIssueResolutions(state)
	if err != nil {
		return nil, err
	}
	theurl := sdk.JoinURL(state.authConfig.APIURL, "/rest/api/3/project/search")
	client := i.httpmanager.New(theurl, nil)
	queryParams := make(url.Values)
	setProjectExpand(queryParams)
	queryParams.Set("typeKey", "software")
	queryParams.Set("status", "live")
	queryParams.Set("maxResults", "100") // 100 is the max, 50 is the default
	var count int
	customerID := state.export.CustomerID()
	started := time.Now()
	savedProjects := make(map[string]*sdk.WorkProject)
	projectSort := make(map[string]int64)
	var hasPreviousProjects bool
	previousProjects := make(map[string]*sdk.WorkProject)
	if state.export.State().Exists(savedPreviousProjectsStateKey) {
		hasPreviousProjects = true
		if _, err := state.export.State().Get(savedPreviousProjectsStateKey, &previousProjects); err != nil {
			return nil, fmt.Errorf("error fetching previous projects state: %w", err)
		}
	}
	for {
		queryParams.Set("startAt", strconv.Itoa(count))
		var resp projectQueryResult
		ts := time.Now()
		r, err := client.Get(&resp, append(state.authConfig.Middleware, sdk.WithGetQueryParameters(queryParams))...)
		if err := i.checkForRateLimit(state.logger, state.export, customerID, err, r.Headers); err != nil {
			return nil, err
		}
		sdk.LogDebug(state.logger, "fetched projects", "len", len(resp.Projects), "total", resp.Total, "count", count, "first", resp.Projects[0].Key, "last", resp.Projects[len(resp.Projects)-1].Key, "duration", time.Since(ts))
		for _, p := range resp.Projects {
			count++
			if p.ProjectTypeKey != "software" {
				sdk.LogInfo(state.logger, "skipping project which isn't a software type", "key", p.Key)
				continue
			}
			if p.Insight != nil {
				if p.Insight.TotalIssueCount == 0 {
					sdk.LogInfo(state.logger, "skipping project because it has no issues yet", "key", p.Key)
					continue
				}
				if p.Insight.LastIssueUpdateTime != "" {
					// try and sort our projects we processed by the ones that have had the most recent changes
					ts, _ := parseTime(p.Insight.LastIssueUpdateTime)
					projectSort[p.ID] = sdk.TimeToEpoch(ts)
					sdk.LogDebug(state.logger, fmt.Sprintf("project %s (%s) total issues=%d, last issue time=%v", p.Key, p.ID, p.Insight.TotalIssueCount, p.Insight.LastIssueUpdateTime))
				}
			}
			issueTypes, err := i.fetchIssueTypesForProject(state, p.ID)
			if err != nil {
				return nil, err
			}
			project, err := p.ToModel(customerID, state.integrationInstanceID, state.authConfig.WebsiteURL, issueTypes, resolutions)
			if err != nil {
				return nil, err
			}
			entityID := state.authConfig.APIURL
			if state.config.Exclusions != nil {
				if state.config.Exclusions.Matches(entityID, p.Key) {
					sdk.LogInfo(state.logger, "marking excluded project inactive: "+p.Name, "id", p.ID, "key", p.Key)
					project.Active = false
				}
			}
			if state.config.Inclusions != nil {
				if !state.config.Inclusions.Matches(entityID, p.Name) && !state.config.Inclusions.Matches(entityID, p.Key) && !state.config.Inclusions.Matches(entityID, p.ID) {
					sdk.LogInfo(state.logger, "marking not included project inactive: "+p.Name, "id", p.ID, "key", p.Key)
					project.Active = false
				}
			}
			if project.Active {
				// do this out here so we dont need the state in createProjectCapability
				getCreateMeta := func() (*projectIssueCreateMeta, error) {
					meta, err := i.fetchIssueCreateMeta(state, []string{project.RefID})
					if err != nil {
						return nil, err
					}
					if len(meta) == 0 {
						return nil, nil
					}
					createMeta := meta[0]
					return &createMeta, nil
				}
				capability, err := i.createProjectCapability(state.export.State(), p, project, getCreateMeta, state.historical)
				if err != nil {
					return nil, err
				}
				if capability != nil {
					// possible to be nil if already processed
					if err := state.pipe.Write(capability); err != nil {
						return nil, err
					}
				}
			}
			savedProjects[p.ID] = project
		}
		if count >= resp.Total {
			break
		}
	}
	if hasPreviousProjects {
		for id, project := range previousProjects {
			p := savedProjects[id]
			if p == nil {
				// not found or now it's excluded, in either case we need to deactivate it
				project.Active = false
				sdk.LogInfo(state.logger, "marking project as inactive since it was exported previously but not included now", "id", id, "key", project.Identifier)
			}
		}
	}
	// we have to do this after we pull all the projects so we can determine if we have old projects
	// that are no longer active
	keys := make([]string, 0)
	var active int
	for key, project := range savedProjects {
		if err := state.pipe.Write(project); err != nil {
			return nil, err
		}
		if project.Active {
			keys = append(keys, key)
			state.stats.incProject()
			active++
		}
	}

	// sort the projects by most recently modified so we process them in that order
	if len(projectSort) > 0 {
		sort.Slice(keys, func(i, j int) bool {
			a := projectSort[keys[i]]
			b := projectSort[keys[j]]
			return a > b
		})
	}

	// save the state so we can check the next time
	if err := state.export.State().Set(savedPreviousProjectsStateKey, savedProjects); err != nil {
		return nil, fmt.Errorf("error saving projects state: %w", err)
	}

	sdk.LogInfo(state.logger, "export projects completed", "duration", time.Since(started), "count", len(savedProjects), "active", active)
	return keys, nil
}

type issueTransitionSource struct {
	Transitions []transitionSource `json:"transitions"`
}

func (i *JiraIntegration) fetchIssueTransitions(logger sdk.Logger, control sdk.Control, authConfig authConfig, customerID string, issueRefID string) ([]sdk.WorkIssueTransitions, error) {
	theurl := sdk.JoinURL(authConfig.APIURL, "/rest/api/3/issue", issueRefID, "/transitions")
	client := i.httpmanager.New(theurl, nil)
	params := url.Values{}
	params.Add("expand", "transitions")
	var resp issueTransitionSource
	r, err := client.Get(&resp, append(authConfig.Middleware, sdk.WithGetQueryParameters(params))...)
	if err := i.checkForRateLimit(logger, control, customerID, err, r.Headers); err != nil {
		if r.StatusCode == http.StatusNotFound {
			sdk.LogWarn(logger, "transitions endpoint returned 404 for issue", "error_body", string(r.Body), "issue", issueRefID)
			return nil, nil
		}
		return nil, err
	}
	return makeTransitions("", resp.Transitions), nil
}

type issuesErr struct {
	ErrorMessages []string `json:"errorMessages"`
}

var invalidProjectRE = regexp.MustCompile(`A value with ID '(\w+)' does not exist for the field 'project`)

func (i *issuesErr) getInvalidProjects() []string {
	var projectIDs []string
	for _, errorMsg := range i.ErrorMessages {
		if invalidProjectRE.MatchString(errorMsg) {
			id := invalidProjectRE.FindStringSubmatch(errorMsg)[1]
			projectIDs = append(projectIDs, id)
		}
	}
	return projectIDs
}

func toIssueError(responseBody []byte) (issuesErr, bool) {
	var ie issuesErr
	if err := json.Unmarshal(responseBody, &ie); err != nil {
		return issuesErr{}, false
	}
	if len(ie.ErrorMessages) == 0 {
		return issuesErr{}, false
	}
	return ie, true
}

func issueSearchJQL(projectKeys []string, fromTime time.Time) string {
	jql := "project in (" + strings.Join(projectKeys, ",") + ") "
	if !fromTime.IsZero() {
		s := relativeDuration(time.Since(fromTime))
		jql += fmt.Sprintf(`AND (created >= "%s" or updated >= "%s") `, s, s)
	}
	jql += "ORDER BY updated DESC" // search for the most recent changes first
	return jql
}

func (i *JiraIntegration) fetchIssuesPaginated(state *state, fromTime time.Time, customfields map[string]customField, projectKeys []string) error {
	theurl := sdk.JoinURL(state.authConfig.APIURL, "/rest/api/3/search")
	client := i.httpmanager.New(theurl, nil)
	queryParams := make(url.Values)
	queryParams.Set("expand", "changelog,fields,comments,transitions")
	queryParams.Set("fields", "*navigable,attachment")
	queryParams.Set("jql", issueSearchJQL(projectKeys, fromTime))
	queryParams.Set("maxResults", "100") // 100 is the max, 50 is the default
	var count int
	customerID := state.export.CustomerID()
	started := time.Now()
	for {
		queryParams.Set("startAt", strconv.Itoa(count))
		var resp issueQueryResult
		ts := time.Now()
		r, err := client.Get(&resp, append(state.authConfig.Middleware, sdk.WithGetQueryParameters(queryParams))...)
		if err := i.checkForRateLimit(state.logger, state.export, customerID, err, r.Headers); err != nil {
			if issueError, ok := toIssueError(r.Body); ok {
				invalidProjectIDs := issueError.getInvalidProjects()
				if len(invalidProjectIDs) > 0 {
					queryParams.Set("jql", issueSearchJQL(removeKeys(projectKeys, invalidProjectIDs), fromTime))
					sdk.LogInfo(state.logger, "found invalid projects, removing from query", "projects", invalidProjectIDs)
					continue
				}
			}
			return fmt.Errorf("error fetching issues: %w", err)
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
			issue, comments, err := i.ToModel(customerID, state.integrationInstanceID, state.issueIDManager, state.sprintManager, state.userManager, customfields, state.authConfig.WebsiteURL, true)
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
		// after the first page, go ahead and flush the data
		if count == 0 {
			state.pipe.Flush()
		}
		count += len(resp.Issues)
		if count >= resp.Total {
			break
		}
	}
	sdk.LogInfo(state.logger, "export issues completed", "duration", time.Since(started), "count", count)
	return nil
}

// configIdentifier saves us from passing an identifier and a config, since most implementations
// of sdk.Identifier also include a Config method
type configIdentifier interface {
	sdk.Identifier
	Config() sdk.Config
	Logger() sdk.Logger
}

func (i *JiraIntegration) createAuthConfig(ci configIdentifier) (authConfig, error) {
	return i.createAuthConfigFromConfig(ci.Logger(), ci, ci.Config())
}

func (i *JiraIntegration) createAuthConfigFromConfig(logger sdk.Logger, identifier sdk.Identifier, config sdk.Config) (authConfig, error) {
	auth, err := newAuth(logger, i.manager, identifier, i.httpmanager, config)
	if err != nil {
		return authConfig{}, err
	}
	return auth.Apply()
}

func (i *JiraIntegration) newState(logger sdk.Logger, pipe sdk.Pipe, authConfig authConfig, config sdk.Config, historical bool, integrationInstanceID string) *state {
	return &state{
		pipe:                  pipe,
		config:                config,
		authConfig:            authConfig,
		logger:                logger,
		historical:            historical,
		integrationInstanceID: integrationInstanceID,
	}
}

const configKeyLastExportTimestamp = "last_export_ts"

// Export is called to tell the integration to run an export
func (i *JiraIntegration) Export(export sdk.Export) error {
	logger := sdk.LogWith(export.Logger(), "job_id", export.JobID())
	sdk.LogInfo(logger, "export started")
	authConfig, err := i.createAuthConfig(export)
	if err != nil {
		return fmt.Errorf("error creating auth config: %w", err)
	}
	state := i.newState(logger, export.Pipe(), authConfig, export.Config(), export.Historical(), export.IntegrationInstanceID())
	state.manager = i.manager
	state.export = export
	state.stats = &stats{
		started: time.Now(),
	}
	if err := i.installWebHookIfNecessary(logger, export.Config(), export.State(), state.authConfig, export.CustomerID(), export.IntegrationInstanceID()); err != nil {
		return fmt.Errorf("error installing webhooks: %w", err)
	}
	var fromTime time.Time
	var fromTimeStr string
	if export.Historical() {
		sdk.LogInfo(logger, "historical has been requested")
	} else {
		if _, err := export.State().Get(configKeyLastExportTimestamp, &fromTimeStr); err != nil {
			return fmt.Errorf("error getting last export time from state: %w", err)
		}
		if fromTimeStr != "" {
			fromTime, _ = time.Parse(time.RFC3339Nano, fromTimeStr)
			sdk.LogInfo(logger, "will start from a specific timestamp", "time", fromTime)
		} else {
			sdk.LogInfo(logger, "no specific timestamp found, will start from now")
		}
	}
	customfields, err := i.fetchCustomFields(logger, state.export, export.CustomerID(), state.authConfig)
	if err != nil {
		return fmt.Errorf("error fetching custom fields: %w", err)
	}
	state.sprintManager = newSprintManager(export.CustomerID(), state.pipe, state.stats, export.IntegrationInstanceID(), state.authConfig.SupportsAgileAPI)
	state.userManager = newUserManager(export.CustomerID(), state.authConfig.WebsiteURL, state.pipe, state.stats, export.IntegrationInstanceID())
	state.issueIDManager = newIssueIDManager(logger, i, state.export, state.pipe, state.sprintManager, state.userManager, customfields, state.authConfig, state.stats)
	if err := i.processWorkConfig(logger, state.config, state.pipe, export.State(), export.CustomerID(), export.IntegrationInstanceID(), export.Historical()); err != nil {
		return err
	}
	projectKeys, err := i.fetchProjectsPaginated(state)
	if err != nil {
		return fmt.Errorf("error fetching projects: %w", err)
	}
	if len(projectKeys) == 0 {
		sdk.LogInfo(logger, "no projects found to export")
	} else {
		if err := state.sprintManager.init(state); err != nil {
			return fmt.Errorf("error in sprintmanager: %w", err)
		}
		if err := i.fetchPriorities(state); err != nil {
			return fmt.Errorf("error fetching priorities: %w", err)
		}
		if err := i.fetchTypes(state); err != nil {
			return fmt.Errorf("error fetching types: %w", err)
		}
		if err := i.fetchIssuesPaginated(state, fromTime, customfields, projectKeys); err != nil {
			return fmt.Errorf("error fetching issues: %w", err)
		}
		if err := state.sprintManager.blockForFetchBoards(logger); err != nil {
			return fmt.Errorf("error waiting for fetched sprints: %w", err)
		}
	}
	if err := export.State().Set(configKeyLastExportTimestamp, state.stats.started.Format(time.RFC3339Nano)); err != nil {
		return fmt.Errorf("error writing last export date to state: %w", err)
	}
	state.stats.dump(logger)
	return nil
}
