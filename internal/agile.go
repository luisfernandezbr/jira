package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pinpt/agent.next/sdk"
)

// easyjson:skip
type sprint struct {
	ID            int
	Name          string
	Goal          string
	State         string
	StartDate     time.Time
	EndDate       time.Time
	CompleteDate  time.Time
	OriginBoardID int
}

func (s sprint) ToModel(customerID string, integrationInstanceID string) (*sdk.AgileSprint, error) {
	sprint := &sdk.AgileSprint{}
	sprint.CustomerID = customerID
	sprint.IntegrationInstanceID = sdk.StringPointer(integrationInstanceID)
	sprint.RefID = strconv.Itoa(s.ID)
	sprint.BoardID = sdk.StringPointer(sdk.NewAgileBoardID(customerID, strconv.Itoa(s.OriginBoardID), refType))
	sprint.ID = sdk.NewAgileSprintID(customerID, sprint.RefID, refType)
	sprint.Goal = s.Goal
	sprint.Name = s.Name
	sdk.ConvertTimeToDateModel(s.StartDate, &sprint.StartedDate)
	sdk.ConvertTimeToDateModel(s.EndDate, &sprint.EndedDate)
	sdk.ConvertTimeToDateModel(s.CompleteDate, &sprint.CompletedDate)
	switch s.State {
	case "CLOSED", "closed":
		sprint.Status = sdk.AgileSprintStatusClosed
	case "ACTIVE", "active":
		sprint.Status = sdk.AgileSprintStatusActive
	case "FUTURE", "future":
		sprint.Status = sdk.AgileSprintStatusFuture
	default:
		return nil, fmt.Errorf("invalid status for sprint: %v", s.State)
	}
	return sprint, nil
}

func parseSprints(data string) (res []sprint, _ error) {
	if data == "" {
		return nil, nil
	}
	var values []string
	err := json.Unmarshal([]byte(data), &values)
	if err != nil {
		return nil, err
	}
	for _, v := range values {
		s, err := parseSprint(v)
		if err != nil {
			return nil, err
		}
		res = append(res, s)
	}
	return
}

func parseSprint(data string) (res sprint, _ error) {
	m, err := parseSprintIntoKV(data)
	if err != nil {
		return res, err
	}
	for k := range m {
		m[k] = processNull(m[k])
	}
	if m["id"] != "" {
		res.ID, err = strconv.Atoi(m["id"])
		if err != nil {
			return res, fmt.Errorf("can't parse id field %v", err)
		}
	}
	res.Name = m["name"]
	res.Goal = m["goal"]
	res.State = m["state"]
	res.StartDate, err = parseSprintTime(m["startDate"])
	if err != nil {
		return res, fmt.Errorf("can't parse startDate %v", err)
	}
	res.EndDate, err = parseSprintTime(m["endDate"])
	if err != nil {
		return res, fmt.Errorf("can't parse endDate %v", err)
	}
	res.CompleteDate, err = parseSprintTime(m["completeDate"])
	if err != nil {
		return res, fmt.Errorf("can't parse completeDate %v", err)
	}
	if m["rapidViewId"] != "" {
		res.OriginBoardID, err = strconv.Atoi(m["rapidViewId"])
		if err != nil {
			return res, fmt.Errorf("can't parse rapidViewId field %v", err)
		}
	}
	return
}

func processNull(val string) string {
	if val == "<null>" {
		return ""
	}
	if val == "\\u003cnull\\u003e" {
		return ""
	}
	return val
}

func parseSprintIntoKV(data string) (map[string]string, error) {
	res := map[string]string{}
	i := strings.Index(data, "[")
	if i == 0 {
		return res, errors.New("can't find [")
	}
	fields := strings.TrimSuffix(data[i+1:], "]")
	if len(fields) == 0 {
		return res, errors.New("no fields")
	}
	re := regexp.MustCompile(`(\w+=.*?)`)
	in := re.FindAllStringIndex(fields, -1)
	for i, tok := range in {
		key := fields[tok[0] : tok[1]-1]
		if i+1 < len(in) {
			val := fields[tok[1] : in[i+1][0]-1]
			res[key] = val
		} else {
			val := fields[tok[1]:]
			res[key] = val
		}
	}
	return res, nil
}

func parseSprintTime(ts string) (time.Time, error) {
	if ts == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, ts)
}

// manager for tracking sprint data as we process issues
// easyjson:skip
type sprintManager struct {
	sprints               map[int]bool
	customerID            string
	pipe                  sdk.Pipe
	stats                 *stats
	integrationInstanceID string
	usingAgileAPI         bool
	async                 sdk.Async
}

func (m *sprintManager) emit(s sprint) error {
	if m.usingAgileAPI {
		return nil // we already fetched them in this case
	}
	if !m.sprints[s.ID] {
		m.sprints[s.ID] = true
		o, err := s.ToModel(m.customerID, m.integrationInstanceID)
		if err != nil {
			return err
		}
		m.stats.incSprint()
		return m.pipe.Write(o)
	}
	return nil
}

// easyjson:skip
type issueSprint struct {
	ID     int
	Goal   string
	Closed bool
}

// easyjson:skip
type boardIssue struct {
	ID        string
	RefID     string
	StatusID  string
	ProjectID string
	Sprints   map[int]*issueSprint
}

func (m *sprintManager) fetchBoardIssues(state *state, boardID int, boardtype string, typestr string) ([]boardIssue, error) {
	theurl := sdk.JoinURL(state.authConfig.APIURL, fmt.Sprintf("/rest/agile/1.0/board/%d/%s", boardID, typestr))
	client := state.manager.HTTPManager().New(theurl, nil)
	var resp struct {
		StartAt    int `json:"startAt"`
		MaxResults int `json:"maxResults"`
		Total      int `json:"total"`
		Issues     []struct {
			ID     string `json:"id"`
			Fields struct {
				Project struct {
					ID string `json:"id"`
				} `json:"project"`
				Status struct {
					ID string `json:"id"`
				} `json:"status"`
				Sprint *struct {
					ID   int    `json:"id"`
					Goal string `json:"goal"`
				}
				ClosedSprints []struct {
					ID   int    `json:"id"`
					Goal string `json:"goal"`
				}
			} `json:"fields"`
		} `json:"issues"`
	}
	var startAt int
	customerID := state.export.CustomerID()
	ts := time.Now()
	var count int
	qs := make(url.Values)
	qs.Set("maxResults", "100")
	qs.Set("fields", "id,project,status,sprint,closedSprints")
	issueids := make([]boardIssue, 0)
	for {
		qs.Set("startAt", strconv.Itoa(startAt))
		r, err := client.Get(&resp, append(state.authConfig.Middleware, sdk.WithGetQueryParameters(qs))...)
		// this means no issues for the sprint
		if r != nil && (r.StatusCode == http.StatusNotFound || r.StatusCode == http.StatusBadRequest) {
			sdk.LogDebug(state.logger, fmt.Sprintf("skipping issues for board %s (%s) for id %d (status code=%d)", typestr, boardtype, boardID, r.StatusCode))
			return nil, nil
		}
		if err != nil {
			return nil, fmt.Errorf("error fetching agile board %d issues: %w", boardID, err)
		}
		for _, issue := range resp.Issues {
			sprints := make(map[int]*issueSprint, 0)
			if issue.Fields.Sprint != nil {
				sprints[issue.Fields.Sprint.ID] = &issueSprint{
					ID:     issue.Fields.Sprint.ID,
					Goal:   issue.Fields.Sprint.Goal,
					Closed: false,
				}
			}
			for _, s := range issue.Fields.ClosedSprints {
				sprints[s.ID] = &issueSprint{
					ID:     s.ID,
					Goal:   s.Goal,
					Closed: true,
				}
			}
			issueids = append(issueids, boardIssue{
				ID:        sdk.NewWorkIssueID(customerID, issue.ID, refType),
				RefID:     issue.ID,
				ProjectID: sdk.NewWorkProjectID(customerID, issue.Fields.Project.ID, refType),
				StatusID:  sdk.NewWorkIssueStatusID(customerID, refType, issue.Fields.Status.ID),
				Sprints:   sprints,
			})
		}
		startAt += len(resp.Issues)
		count += len(resp.Issues)
		if count >= resp.Total {
			// jira is so dumb and doesn't have isLast for this api like others
			break
		}
	}
	sdk.LogDebug(state.logger, "fetched agile board issues", "board", boardID, "len", count, "duration", time.Since(ts))
	return issueids, nil
}

// easyjson:skip
type sprintIssue struct {
	ID        string
	ProjectID string
	Goal      string
	Status    string
}

func (m *sprintManager) fetchSprintIssues(state *state, sprintID int) ([]sprintIssue, error) {
	theurl := sdk.JoinURL(state.authConfig.APIURL, fmt.Sprintf("/rest/agile/1.0/sprint/%d/issue", sprintID))
	client := state.manager.HTTPManager().New(theurl, nil)
	var resp struct {
		StartAt    int `json:"startAt"`
		MaxResults int `json:"maxResults"`
		Total      int `json:"total"`
		Issues     []struct {
			ID     string `json:"id"`
			Fields struct {
				Project struct {
					ID string `json:"id"`
				} `json:"project"`
				Sprint struct {
					Goal string `json:"goal"`
				}
				Status struct {
					ID string `json:"id"`
				} `json:"status"`
			} `json:"fields"`
		} `json:"issues"`
	}
	var startAt int
	ts := time.Now()
	var count int
	qs := make(url.Values)
	qs.Set("maxResults", "100")
	issues := make([]sprintIssue, 0)
	customerID := state.export.CustomerID()
	for {
		qs.Set("startAt", strconv.Itoa(startAt))
		r, err := client.Get(&resp, append(state.authConfig.Middleware, sdk.WithGetQueryParameters(qs))...)
		// this means no sprints for the board
		if r != nil && r.StatusCode == http.StatusNotFound {
			return nil, nil
		}
		if err != nil {
			return nil, fmt.Errorf("error fetching agile sprints: %w", err)
		}
		for _, issue := range resp.Issues {
			issues = append(issues, sprintIssue{
				ID:        sdk.NewWorkIssueID(customerID, issue.ID, refType),
				ProjectID: sdk.NewWorkProjectID(customerID, issue.Fields.Project.ID, refType),
				Goal:      issue.Fields.Sprint.Goal,
				Status:    sdk.NewWorkIssueStatusID(customerID, refType, issue.Fields.Status.ID),
			})
		}
		startAt += len(resp.Issues)
		count += len(resp.Issues)
		if count >= resp.Total {
			break
		}
	}
	sdk.LogDebug(state.logger, "fetched agile sprint issues", "id", sprintID, "len", count, "duration", time.Since(ts))
	return issues, nil
}

func (m *sprintManager) fetchSprint(state *state, sprintID int, boardID string, boardProjectKeys map[int]string, statusmapping map[string]*int, columncount int) error {
	theurl := sdk.JoinURL(state.authConfig.APIURL, fmt.Sprintf("/rest/agile/1.0/sprint/%d", sprintID))
	client := state.manager.HTTPManager().New(theurl, nil)
	var s struct {
		Goal         string    `json:"goal"`
		State        string    `json:"state"`
		Name         string    `json:"name"`
		StartDate    time.Time `json:"startDate,omitempty"`
		EndDate      time.Time `json:"endDate,omitempty"`
		CompleteDate time.Time `json:"completeDate,omitempty"`
		BoardID      int       `json:"originBoardId"`
	}
	ts := time.Now()
	_, err := client.Get(&s, state.authConfig.Middleware...)
	if err != nil {
		return err
	}
	if boardProjectKeys[s.BoardID] == "" {
		// this is an old sprint which was deleted or the board doesn't exist
		sdk.LogDebug(state.logger, fmt.Sprintf("skipping sprint (%v/%s) since the board id (%v) couldn't be found", sprintID, s.Name, s.BoardID))
		return nil
	}
	var sprint sdk.AgileSprint
	sprint.CustomerID = state.export.CustomerID()
	sprint.IntegrationInstanceID = sdk.StringPointer(state.export.IntegrationInstanceID())
	sprint.RefID = strconv.Itoa(sprintID)
	sprint.RefType = refType
	sprint.Name = s.Name
	sprint.ID = sdk.NewAgileSprintID(sprint.CustomerID, sprint.RefID, refType)
	sprint.BoardID = sdk.StringPointer(boardID)
	sprint.Active = true
	sdk.ConvertTimeToDateModel(s.StartDate, &sprint.StartedDate)
	sdk.ConvertTimeToDateModel(s.EndDate, &sprint.EndedDate)
	sdk.ConvertTimeToDateModel(s.CompleteDate, &sprint.CompletedDate)
	switch s.State {
	case "CLOSED", "closed":
		sprint.Status = sdk.AgileSprintStatusClosed
	case "ACTIVE", "active":
		sprint.Status = sdk.AgileSprintStatusActive
	case "FUTURE", "future":
		sprint.Status = sdk.AgileSprintStatusFuture
	default:
	}
	issues, err := m.fetchSprintIssues(state, sprintID)
	if err != nil {
		return err
	}
	sprint.IssueIds = make([]string, 0)
	sprint.Columns = make([]sdk.AgileSprintColumns, 0)
	columns := make([]*sdk.AgileSprintColumns, columncount)
	for i := 0; i < columncount; i++ {
		columns[i] = &sdk.AgileSprintColumns{
			IssueIds: make([]string, 0),
		}
	}
	for _, issue := range issues {
		if sprint.Goal == "" {
			sprint.Goal = issue.Goal
		}
		// for the status id, find the column to place it in
		i := statusmapping[issue.Status]
		if i != nil {
			sprint.IssueIds = append(sprint.IssueIds, issue.ID)
			columns[*i].IssueIds = append(columns[*i].IssueIds, issue.ID)
		}
	}
	for _, c := range columns {
		sprint.Name = c.Name
		sprint.Columns = append(sprint.Columns, *c)
	}
	if sprint.Status == sdk.AgileSprintStatusClosed {
		sprint.URL = completedSprintURL(state.authConfig.WebsiteURL, s.BoardID, boardProjectKeys[s.BoardID], sprintID)
	} else {
		sprint.URL = boardURL(state.authConfig.WebsiteURL, s.BoardID, boardProjectKeys[s.BoardID])
	}
	if err := state.export.State().Set(m.getSprintStateKey(sprintID), sdk.EpochNow()); err != nil {
		return fmt.Errorf("error writing sprint key to state: %w", err)
	}
	if err := state.pipe.Write(&sprint); err != nil {
		return fmt.Errorf("error writing sprint to pipe: %w", err)
	}
	sdk.LogInfo(state.logger, "fetched sprint", "id", sprintID, "duration", time.Since(ts))
	return nil
}

func (m *sprintManager) getSprintStateKey(id int) string {
	return fmt.Sprintf("sprint_%d", id)
}

func (m *sprintManager) fetchSprints(state *state, boardID int, projectKey string, projectID string) ([]int, error) {
	theurl := sdk.JoinURL(state.authConfig.APIURL, fmt.Sprintf("/rest/agile/1.0/board/%d/sprint", boardID))
	client := state.manager.HTTPManager().New(theurl, nil)
	var resp struct {
		MaxResults int  `json:"maxResults"`
		StartAt    int  `json:"startAt"`
		Total      int  `json:"total"`
		IsLast     bool `json:"isLast"`
		Values     []struct {
			ID    int    `json:"id"`
			State string `json:"state"`
		} `json:"values"`
	}
	var startAt int
	ts := time.Now()
	var count int
	qs := make(url.Values)
	qs.Set("maxResults", "100")
	qs.Set("state", "future,active,closed")
	sprintids := make([]int, 0)
	oldids := make([]int, 0)
	for {
		qs.Set("startAt", strconv.Itoa(startAt))
		r, err := client.Get(&resp, append(state.authConfig.Middleware, sdk.WithGetQueryParameters(qs))...)
		// this means no sprints for the board
		if r != nil && r.StatusCode == http.StatusNotFound {
			return nil, nil
		}
		if err != nil {
			return nil, fmt.Errorf("error fetching agile sprints: %w", err)
		}
		for _, s := range resp.Values {
			if s.State == "closed" && !state.historical {
				if state.export.State().Exists(m.getSprintStateKey(s.ID)) {
					sdk.LogDebug(state.logger, "skipping sprint since we've already processed it", "id", s.ID)
					continue
				}
			}
			if s.State == "closed" {
				oldids = append(oldids, s.ID)
			} else {
				sprintids = append(sprintids, s.ID)
			}
		}
		if resp.IsLast {
			break
		}
		startAt += len(resp.Values)
		count += len(resp.Values)
	}
	sdk.LogDebug(state.logger, "fetched agile sprints", "board", boardID, "len", count, "duration", time.Since(ts))
	// return the newer ones before the older ones
	return append(sprintids, oldids...), nil
}

// easyjson:skip
type boardColumn struct {
	Name      string
	StatusIDs []string
}

func (m *sprintManager) fetchBoardConfig(state *state, boardID int) ([]boardColumn, error) {
	theurl := sdk.JoinURL(state.authConfig.APIURL, fmt.Sprintf("/rest/agile/1.0/board/%d/configuration", boardID))
	client := state.manager.HTTPManager().New(theurl, nil)
	ts := time.Now()
	var count int
	var resp struct {
		ColumnConfig struct {
			Columns []struct {
				Name     string `json:"name"`
				Statuses []struct {
					ID string `json:"id"`
				} `json:"statuses"`
			} `json:"columns"`
		} `json:"columnConfig"`
	}
	_, err := client.Get(&resp, state.authConfig.Middleware...)
	if err != nil {
		return nil, fmt.Errorf("error fetching agile board %d config: %w", boardID, err)
	}
	customerID := state.export.CustomerID()
	columns := make([]boardColumn, 0)
	for _, c := range resp.ColumnConfig.Columns {
		statusids := make([]string, 0)
		for _, s := range c.Statuses {
			statusids = append(statusids, sdk.NewWorkIssueStatusID(customerID, refType, s.ID))
		}
		columns = append(columns, boardColumn{
			Name:      c.Name,
			StatusIDs: statusids,
		})
	}
	sdk.LogDebug(state.logger, "fetched agile board config", "id", boardID, "len", count, "duration", time.Since(ts))
	return columns, nil
}

// easyjson:skip
type boardDetail struct {
	ID         int
	Name       string
	Type       string
	ProjectKey string
	ProjectID  string
}

func (m *sprintManager) fetchBoards(state *state) error {
	theurl := sdk.JoinURL(state.authConfig.APIURL, "/rest/agile/1.0/board")
	client := state.manager.HTTPManager().New(theurl, nil)
	var startAt int
	ts := time.Now()
	var count int
	var resp struct {
		MaxResults int  `json:"maxResults"`
		StartAt    int  `json:"startAt"`
		Total      int  `json:"total"`
		IsLast     bool `json:"isLast"`
		Values     []struct {
			ID       int    `json:"id"`
			Self     string `json:"self"`
			Name     string `json:"name"`
			Type     string `json:"type"`
			Location struct {
				ID         int    `json:"projectId"`
				ProjectKey string `json:"projectKey"`
			} `json:"location"`
		} `json:"values"`
	}
	qs := make(url.Values)
	qs.Set("maxResults", "100")
	customerID := state.export.CustomerID()
	boardProjectKeys := make(map[int]string)
	boards := make([]boardDetail, 0)
	for {
		qs.Set("startAt", strconv.Itoa(startAt))
		_, err := client.Get(&resp, append(state.authConfig.Middleware, sdk.WithGetQueryParameters(qs))...)
		if err != nil {
			return fmt.Errorf("error fetching agile boards: %w", err)
		}
		for _, board := range resp.Values {
			boardProjectKeys[board.ID] = board.Location.ProjectKey
			boards = append(boards, boardDetail{
				ID:         board.ID,
				Name:       board.Name,
				Type:       board.Type,
				ProjectKey: board.Location.ProjectKey,
				ProjectID:  sdk.NewWorkProjectID(customerID, strconv.Itoa(board.Location.ID), refType),
			})
		}
		if resp.IsLast {
			break
		}
		startAt += len(resp.Values)
		count += len(resp.Values)
	}
	// sort id in descending order since newer boards have bigger id than older ones
	sort.Slice(boards, func(i, j int) bool {
		left, right := boards[i], boards[j]
		return left.ID > right.ID
	})

	var lock sync.Mutex
	sprintids := make(map[int]bool)

	for _, _board := range boards {
		if _board.Type != "scrum" && _board.Type != "kanban" {
			// we only support scrum and kanban boards
			continue
		}
		if _board.ProjectKey == "" {
			// this is an orphaned board (probably created by someone no longer around)
			continue
		}

		var board = _board
		m.async.Do(func() error {

			// fetch the board config to get the columns
			columns, err := m.fetchBoardConfig(state, board.ID)
			if err != nil {
				return err
			}

			var theboard sdk.AgileBoard
			theboard.ID = sdk.NewAgileBoardID(customerID, strconv.Itoa(board.ID), refType)
			theboard.Active = true
			theboard.CustomerID = customerID
			theboard.RefType = refType
			theboard.RefID = strconv.Itoa(board.ID)
			theboard.IntegrationInstanceID = sdk.StringPointer(state.integrationInstanceID)
			theboard.Name = board.Name
			theboard.Columns = make([]sdk.AgileBoardColumns, 0)

			// build the kanban columnss
			statusmapping := make(map[string]*int)
			isScrum := board.Type == "scrum"
			var hasBacklogColumn, fetchBacklog bool
			var colindex, backlogIndex int
			filteredcolumns := make([]boardColumn, 0)
			for i, col := range columns {
				if col.Name == "Backlog" {
					backlogIndex = i
					hasBacklogColumn = true
					fetchBacklog = len(col.StatusIDs) > 0
					if isScrum {
						continue // we need the backlog for kanban below
					}
				}
				if len(col.StatusIDs) == 0 {
					continue
				}
				theboard.Columns = append(theboard.Columns, sdk.AgileBoardColumns{
					Name: col.Name,
				})
				var c = colindex
				for _, id := range col.StatusIDs {
					statusmapping[id] = &c
				}
				filteredcolumns = append(filteredcolumns, col)
				colindex++
			}

			// if we don't have a backlog column and it's scrum we can fetch the backlog
			if !hasBacklogColumn && isScrum {
				fetchBacklog = true
			}

			theboard.BacklogIssueIds = make([]string, 0)

			if fetchBacklog {
				// fetch the backlog for the board
				backlogids, err := m.fetchBoardIssues(state, board.ID, board.Type, "backlog")
				if err != nil {
					return err
				}
				for _, b := range backlogids {
					theboard.BacklogIssueIds = append(theboard.BacklogIssueIds, sdk.NewWorkIssueID(customerID, b.RefID, refType))
				}
			} else {
				sdk.LogDebug(state.logger, "skipping backlog for board since its not supported", "id", board.ID)
			}

			switch board.Type {
			case "scrum":
				theboard.Type = sdk.AgileBoardTypeScrum
				sids, err := m.fetchSprints(state, board.ID, board.ProjectKey, board.ProjectID)
				if err != nil {
					return fmt.Errorf("error fetching sprints for board id %d. %w", board.ID, err)
				}
				for _, sid := range sids {
					lock.Lock()
					f := sprintids[sid]
					if !f {
						sprintids[sid] = true
					}
					lock.Unlock()
					if f {
						// already processed it since we have same sprint pointing at other boards
						continue
					}
					if err := m.fetchSprint(state, sid, theboard.ID, boardProjectKeys, statusmapping, len(filteredcolumns)); err != nil {
						return err
					}
				}
			case "kanban":
				theboard.Type = sdk.AgileBoardTypeKanban
				var kanban sdk.AgileKanban
				kanban.Active = true
				kanban.CustomerID = customerID
				kanban.IntegrationInstanceID = sdk.StringPointer(state.export.IntegrationInstanceID())
				kanban.RefID = strconv.Itoa(board.ID)
				kanban.RefType = refType
				kanban.IssueIds = make([]string, 0)
				kanban.Columns = make([]sdk.AgileKanbanColumns, 0)
				boardcolumns := make([]*sdk.AgileKanbanColumns, 0)
				mappings := make(map[string]*sdk.AgileKanbanColumns)
				for _, c := range filteredcolumns {
					bc := &sdk.AgileKanbanColumns{
						IssueIds: make([]string, 0),
						Name:     c.Name,
					}
					boardcolumns = append(boardcolumns, bc)
					for _, id := range c.StatusIDs {
						mappings[id] = bc
					}
				}
				// fetch all the board issues and assign them to the right columns
				boardissues, err := m.fetchBoardIssues(state, board.ID, board.Type, "issue")
				if err != nil {
					return fmt.Errorf("error fetching kanban issues for board id %d. %w", board.ID, err)
				}
				// attach each issue to the right board column
				for _, bi := range boardissues {
					boardcolumn := mappings[bi.StatusID]
					if boardcolumn == nil {
						sdk.LogError(state.logger, "couldn't find board column for ("+bi.StatusID+") issue", "issue", bi.ID)
						continue
					}
					boardcolumn.IssueIds = append(boardcolumn.IssueIds, bi.ID)
					kanban.IssueIds = append(kanban.IssueIds, bi.ID)
				}
				// add the columns
				var startat int
				if hasBacklogColumn && fetchBacklog && backlogIndex == 0 {
					// for kanban boards you can setup the column such that the first one is your backlog
					// https://support.atlassian.com/jira-software-cloud/docs/use-your-kanban-backlog/
					// we want to skip this column since we already have a separate backlog array
					startat = 1
				}
				for _, c := range boardcolumns[startat:] {
					kanban.Columns = append(kanban.Columns, *c)
				}
				// the first column in kanban is always the backlog
				theboard.BacklogIssueIds = boardcolumns[0].IssueIds
				kanban.URL = boardURL(state.authConfig.WebsiteURL, board.ID, board.ProjectKey)
				kanban.ID = sdk.NewAgileKanbanID(customerID, strconv.Itoa(board.ID), refType)
				kanban.BoardID = theboard.ID

				// send it off 🚢
				if err := state.pipe.Write(&kanban); err != nil {
					return err
				}
			}
			// now send the board details
			if err := state.pipe.Write(&theboard); err != nil {
				return err
			}
			return nil
		})
	}
	sdk.LogDebug(state.logger, "fetched agile boards", "len", count, "duration", time.Since(ts))
	return nil
}

func (m *sprintManager) blockForFetchBoards(logger sdk.Logger) error {
	defer sdk.LogDebug(logger, "blockForFetchBoards completed")
	sdk.LogDebug(logger, "blockForFetchBoards")
	return m.async.Wait()
}

func (m *sprintManager) init(state *state) error {
	if !m.usingAgileAPI {
		return nil
	}
	// if using the Agile API we can go fetch all the data from it instead of parsing issues for it
	started := time.Now()
	if err := m.fetchBoards(state); err != nil {
		return err
	}
	sdk.LogDebug(state.logger, "fetched agile boards", "duration", time.Since(started))
	return nil
}

func newSprintManager(customerID string, pipe sdk.Pipe, stats *stats, integrationInstanceID string, usingAgileAPI bool) *sprintManager {
	return &sprintManager{
		sprints:               make(map[int]bool),
		customerID:            customerID,
		pipe:                  pipe,
		stats:                 stats,
		integrationInstanceID: integrationInstanceID,
		usingAgileAPI:         usingAgileAPI,
		async:                 sdk.NewAsync(10),
	}
}
