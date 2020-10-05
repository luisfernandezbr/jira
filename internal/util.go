package internal

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/pinpt/agent/v4/sdk"
)

func relativeDuration(d time.Duration) string {
	d = d.Round(time.Minute)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	if h > 0 {
		return fmt.Sprintf("-%dm", h*60+m) // convert to minutes
	}
	if m == 0 {
		return "-1m" // always return at least 1m ago
	}
	return fmt.Sprintf("-%dm", m)
}

// jira format: 2019-07-12T22:32:50.376+0200
const jiraTimeFormat = "2006-01-02T15:04:05.999Z0700"

func parseTime(ts string) (time.Time, error) {
	if ts == "" {
		return time.Time{}, nil
	}
	return time.Parse(jiraTimeFormat, ts)
}

func parsePlannedDate(ts string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", ts, time.UTC)
}

func projectURL(websiteURL, projectKey string) string {
	return sdk.JoinURL(websiteURL, "browse", projectKey)
}

func issueURL(websiteURL, issueKey string) string {
	return sdk.JoinURL(websiteURL, "browse", issueKey)
}

func issueCommentURL(websiteURL, issueKey string, commentID string) string {
	// looks like: https://pinpt-hq.atlassian.net/browse/DE-2194?focusedCommentId=17692&page=com.atlassian.jira.plugin.system.issuetabpanels%3Acomment-tabpanel#comment-17692
	return sdk.JoinURL(websiteURL, "browse", issueKey+fmt.Sprintf("?focusedCommentId=%s&page=com.atlassian.jira.plugin.system.issuetabpanels%%3Acomment-tabpanel#comment-%s", commentID, commentID))
}

func completedSprintURL(websiteURL string, boardID int, projectKey string, sprintID int) string {
	//https://pinpt-hq.atlassian.net/secure/RapidBoard.jspa?rapidView=28&projectKey=APP&view=reporting&chart=sprintRetrospective&sprint=164
	return sdk.JoinURL(websiteURL, fmt.Sprintf("secure/RapidBoard.jspa?rapidView=%d&projectKey=%s&view=reporting&chart=sprintRetrospective&sprint=%v", boardID, projectKey, sprintID))
}

func boardURL(websiteURL string, boardID int, projectKey string) string {
	//looks like: https://pinpt-hq.atlassian.net/secure/RapidBoard.jspa?rapidView=80
	return sdk.JoinURL(websiteURL, fmt.Sprintf("secure/RapidBoard.jspa?rapidView=%d&projectKey=%s", boardID, projectKey))
}

var sprintRegexp = regexp.MustCompile(`com\.atlassian\.greenhopper\.service\.sprint\.Sprint@.+?\[*id=(\d+)`)

func extractPossibleSprintID(v string) string {
	matches := sprintRegexp.FindStringSubmatch(sdk.Stringify(v))
	if len(matches) == 0 {
		return ""
	}
	return matches[1]
}

var imgRegexp = regexp.MustCompile(`(?s)<span class="image-wrap"[^\>]*>(.*?src\=(?:\"|\')(.+?)(?:\"|\').*?)<\/span>`)

var emoticonRegexp = regexp.MustCompile(`<img class="emoticon" src="([^"]*)"[^>]*\/>`)

// we need to pull out the HTML and parse it so we can properly display it in the application. the HTML will
// have a bunch of stuff we need to cleanup for rendering in our application such as relative urls, etc. we
// clean this up here and fix any urls and weird html issues
func adjustRenderedHTML(websiteURL, data string) string {
	if data == "" {
		return ""
	}

	for _, m := range imgRegexp.FindAllStringSubmatch(data, -1) {
		url := m[2] // this is the image group
		// if a relative url but not a // meaning protocol to the page, then make an absolute url to the server
		if strings.HasPrefix(url, "/") && !strings.HasPrefix(url, "//") {
			// replace the relative url with an absolute url. the app will handle the case where the app
			// image is unreachable because the server is behind a corporate firewall and the user isn't on
			// the network when viewing it
			url = sdk.JoinURL(websiteURL, url)
		}
		// replace the <span> wrapped thumbnail junk with just a simple image tag
		newval := strings.Replace(m[0], m[1], `<img src="`+url+`" />`, 1)
		data = strings.ReplaceAll(data, m[0], newval)
	}

	for _, m := range emoticonRegexp.FindAllStringSubmatch(data, -1) {
		url := m[1]
		if strings.HasPrefix(url, "/") && !strings.HasPrefix(url, "//") {
			url = sdk.JoinURL(websiteURL, url)
		}
		newval := strings.Replace(m[0], m[1], url, 1)
		data = strings.ReplaceAll(data, m[0], newval)
	}

	// we apply a special tag here to allow the front-end to handle integration specific data for the integration in
	// case we need to do integration specific image handling
	return `<div class="source-jira">` + strings.TrimSpace(data) + `</div>`
}

// easyjson:skip
type stats struct {
	started       time.Time
	issueCount    int
	commentCount  int
	projectCount  int
	priorityCount int
	typeCount     int
	sprintCount   int
	userCount     int
	mu            sync.Mutex
}

func (s *stats) dump(logger sdk.Logger) {
	sdk.LogInfo(logger, "export stats", "issues", s.issueCount, "comments", s.commentCount, "projects", s.projectCount, "priorities", s.priorityCount, "types", s.typeCount, "sprints", s.sprintCount, "users", s.userCount, "duration", time.Since(s.started))
}

func (s *stats) incIssue() {
	s.mu.Lock()
	s.issueCount++
	s.mu.Unlock()
}

func (s *stats) incComment() {
	s.mu.Lock()
	s.commentCount++
	s.mu.Unlock()
}

func (s *stats) incProject() {
	s.mu.Lock()
	s.projectCount++
	s.mu.Unlock()
}

func (s *stats) incPriority() {
	s.mu.Lock()
	s.priorityCount++
	s.mu.Unlock()
}

func (s *stats) incType() {
	s.mu.Lock()
	s.typeCount++
	s.mu.Unlock()
}

func (s *stats) incSprint() {
	s.mu.Lock()
	s.sprintCount++
	s.mu.Unlock()
}

func (s *stats) incUser() {
	s.mu.Lock()
	s.userCount++
	s.mu.Unlock()
}

// state is everything you ever wanted during an export ... lol
// easyjson:skip
type state struct {
	logger                sdk.Logger
	export                sdk.Export
	pipe                  sdk.Pipe
	config                sdk.Config
	stats                 *stats
	authConfig            authConfig
	sprintManager         *sprintManager
	userManager           UserManager
	issueIDManager        *issueIDManager
	manager               sdk.Manager
	client                sdk.GraphQLClient
	historical            bool
	integrationInstanceID string
}

type jiraErrResp struct {
	ErrorMessages []string          `json:"errorMessages"`
	Errors        map[string]string `json:"errors"`
}

func getJiraErrorMessage(err error) string {
	if ok, _, r := sdk.IsHTTPError(err); ok {
		var errResp jiraErrResp
		json.NewDecoder(r).Decode(&errResp)
		if len(errResp.ErrorMessages) > 0 {
			return errResp.ErrorMessages[0]
		}
		if len(errResp.Errors) > 0 {
			for k, v := range errResp.Errors {
				// return the first one
				return fmt.Sprintf("%s: %s", k, v)
			}
		}
	}
	return err.Error()
}

func sliceContains(s []string, value string) bool {
	for _, v := range s {
		if v == value {
			return true
		}
	}
	return false
}

// appendUnique will append val(s) to arr if arr does not already contain them
func appendUnique(arr []string, val ...string) []string {
	for _, v := range val {
		if !sliceContains(arr, v) {
			arr = append(arr, v)
		}
	}
	return arr
}

func appendStateArray(state sdk.State, stateKey string, newVals ...string) error {
	var existingInts []string
	found, err := state.Get(stateKey, &existingInts)
	if err != nil {
		return fmt.Errorf("error getting string array (%s) from state: %w", stateKey, err)
	}
	var shouldSave bool
	if found {
		exists := make(map[string]bool)
		for _, existing := range existingInts {
			exists[existing] = true
		}
		for _, newVal := range newVals {
			if !exists[newVal] {
				// if it doesnt exist in the current array, then add to it
				existingInts = append(existingInts, newVal)
				shouldSave = true
			}
		}
	} else {
		existingInts = newVals
		shouldSave = true
	}
	if shouldSave {
		if err := state.Set(stateKey, existingInts); err != nil {
			return fmt.Errorf("error saving string array to state: %w", err)
		}
	}
	return nil
}

func reverseString(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

func removeKeys(s []string, remove []string) (spliced []string) {
	for _, maybe := range s {
		if !sliceContains(remove, maybe) {
			spliced = append(spliced, maybe)
		}
	}
	return
}
