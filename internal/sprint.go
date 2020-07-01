package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pinpt/agent.next/sdk"
)

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

func (s sprint) ToModel(customerID string, integrationInstanceID string) (*sdk.WorkSprint, error) {
	sprint := &sdk.WorkSprint{}
	sprint.CustomerID = customerID
	sprint.IntegrationInstanceID = sdk.StringPointer(integrationInstanceID)
	sprint.RefID = strconv.Itoa(s.ID)
	sprint.ID = sdk.NewWorkSprintID(customerID, sprint.RefID, refType)
	sprint.Goal = s.Goal
	sprint.Name = s.Name
	sdk.ConvertTimeToDateModel(s.StartDate, &sprint.StartedDate)
	sdk.ConvertTimeToDateModel(s.EndDate, &sprint.EndedDate)
	sdk.ConvertTimeToDateModel(s.CompleteDate, &sprint.CompletedDate)
	switch s.State {
	case "CLOSED":
		sprint.Status = sdk.WorkSprintStatusClosed
	case "ACTIVE":
		sprint.Status = sdk.WorkSprintStatusActive
	case "FUTURE":
		sprint.Status = sdk.WorkSprintStatusFuture
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
	for f, re := range sprintREs {
		v := re.FindStringSubmatch(fields)
		if len(v) != 2 {
			continue
		}
		res[f] = v[1]
	}
	return res, nil
}

var sprintREs map[string]*regexp.Regexp

func init() {
	sprintREs = map[string]*regexp.Regexp{}
	for _, f := range []string{"id", "rapidViewId", "state", "name", "startDate", "endDate", "completeDate", "sequence", "goal"} {
		sprintREs[f] = regexp.MustCompile(f + "=([^,]*)")
	}
}

func parseSprintTime(ts string) (time.Time, error) {
	if ts == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, ts)
}

// manager for tracking sprint data as we process issues
type sprintManager struct {
	sprints               map[int]bool
	customerID            string
	pipe                  sdk.Pipe
	stats                 *stats
	integrationInstanceID string
}

func (m *sprintManager) emit(s sprint) error {
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

func newSprintManager(customerID string, pipe sdk.Pipe, stats *stats, integrationInstanceID string) *sprintManager {
	return &sprintManager{
		sprints:               make(map[int]bool),
		customerID:            customerID,
		pipe:                  pipe,
		stats:                 stats,
		integrationInstanceID: integrationInstanceID,
	}
}
