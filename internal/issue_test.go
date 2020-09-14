package internal

import (
	"testing"
	"time"

	"github.com/pinpt/agent.next/sdk"
	"github.com/stretchr/testify/assert"
)

func TestSprintRegexp(t *testing.T) {

	cases := []struct {
		Label string
		In    string
		Want  string
	}{
		{`active`, `com.atlassian.greenhopper.service.sprint.Sprint@75abc849[id=3,rapidViewId=6,state=ACTIVE,name=Sample Sprint 2,goal=<null>,startDate=2017-06-03T12:55:01.165Z,endDate=2017-06-17T13:15:01.165Z,completeDate=<null>,sequence=3]`, "3"},
		{`complete`, `com.atlassian.greenhopper.service.sprint.Sprint@75abc849[id=3,rapidViewId=6,state=COMPLETE,name=Sample Sprint 2,goal=<null>,startDate=2017-06-03T12:55:01.165Z,endDate=2017-06-17T13:15:01.165Z,completeDate=<null>,sequence=3]`, "3"},
		{`closed`, `com.atlassian.greenhopper.service.sprint.Sprint@5562e050[id=123,rapidViewId=28,state=CLOSED,name=App Sprint End Nov 22nd,goal=,startDate=2019-11-12T17:13:19.314Z,endDate=2019-11-23T07:13:00.000Z,completeDate=2019-12-02T16:26:31.394Z,sequence=123]`, "123"},
		{`extra s ??? probably just invalid data`, `com.atlassian.greenhopper.service.sprints.Sprint@75abc849[id=3,rapidViewId=6,state=ACTIVE,name=Sample Sprint 2,goal=<null>,startDate=2017-06-03T12:55:01.165Z,endDate=2017-06-17T13:15:01.165Z,completeDate=<null>,sequence=3]`, ""},
		{`active`, `com.atlassian.greenhopper.service.sprint.Sprint@75abc849[rapidViewId=6,id=3,state=ACTIVE,name=Sample Sprint 2,goal=<null>,startDate=2017-06-03T12:55:01.165Z,endDate=2017-06-17T13:15:01.165Z,completeDate=<null>,sequence=3]`, "3"},
		{`multiline`, "com.atlassian.greenhopper.service.sprint.Sprint@503ddc22[completeDate=2020-07-14T16:28:58.065Z,endDate=2020-07-13T07:29:00.000Z,goal=Have a fun 4th of July!\n\nData Manager Updates\nPlatform Altering and Monitoring,id=191,name=BOLT Sprint Ending 7.13,rapidViewId=79,sequence=191,startDate=2020-06-29T17:58:10.922Z,state=CLOSED]", "191"},
	}

	for _, c := range cases {
		got := extractPossibleSprintID(c.In)
		if got != c.Want {
			t.Errorf("case '%v' wanted `%v` got `%v`", c.Label, c.Want, got)
		}
	}
}

func TestSprintFieldsRegexp(t *testing.T) {

	cases := []struct {
		Label string
		In    string
		Want  string
	}{
		{`goal`, `com.atlassian.greenhopper.service.sprint.Sprint@4ec59d6[completeDate=<null>,endDate=2020-07-06T16:16:00.000Z,goal=Code Complete for Agent 4.0, and something else,id=190,name=Gold Sprint 2,rapidViewId=75,sequence=190,startDate=2020-06-22T16:16:28.226Z,state=ACTIVE]`, "Code Complete for Agent 4.0, and something else"},
		{`state`, `com.atlassian.greenhopper.service.sprint.Sprint@4ec59d6[completeDate=<null>,endDate=2020-07-06T16:16:00.000Z,goal=Code Complete for Agent 4.0, and something else,id=190,name=Gold Sprint 2,rapidViewId=75,sequence=190,startDate=2020-06-22T16:16:28.226Z,state=ACTIVE]`, "ACTIVE"},
		{`endDate`, `com.atlassian.greenhopper.service.sprint.Sprint@4ec59d6[completeDate=<null>,endDate=2020-07-06T16:16:00.000Z,goal=Code Complete for Agent 4.0, and something else,id=190,name=Gold Sprint 2,rapidViewId=75,sequence=190,startDate=2020-06-22T16:16:28.226Z,state=ACTIVE]`, "2020-07-06T16:16:00.000Z"},
		{`startDate`, `com.atlassian.greenhopper.service.sprint.Sprint@4ec59d6[completeDate=<null>,endDate=2020-07-06T16:16:00.000Z,goal=Code Complete for Agent 4.0, and something else,id=190,name=Gold Sprint 2,rapidViewId=75,sequence=190,startDate=2020-06-22T16:16:28.226Z,state=ACTIVE]`, "2020-06-22T16:16:28.226Z"},
		{`rapidViewId`, `com.atlassian.greenhopper.service.sprint.Sprint@4ec59d6[completeDate=<null>,endDate=2020-07-06T16:16:00.000Z,goal=Code Complete for Agent 4.0, and something else,id=190,name=Gold Sprint 2,rapidViewId=75,sequence=190,startDate=2020-06-22T16:16:28.226Z,state=ACTIVE]`, "75"},
		{`sequence`, `com.atlassian.greenhopper.service.sprint.Sprint@4ec59d6[completeDate=<null>,endDate=2020-07-06T16:16:00.000Z,goal=Code Complete for Agent 4.0, and something else,id=190,name=Gold Sprint 2,rapidViewId=75,sequence=190,startDate=2020-06-22T16:16:28.226Z,state=ACTIVE]`, "190"},
		{`name`, `com.atlassian.greenhopper.service.sprint.Sprint@4ec59d6[completeDate=<null>,endDate=2020-07-06T16:16:00.000Z,goal=Code Complete for Agent 4.0, and something else,id=190,name=Gold Sprint 2,rapidViewId=75,sequence=190,startDate=2020-06-22T16:16:28.226Z,state=ACTIVE]`, "Gold Sprint 2"},
		{`id`, `com.atlassian.greenhopper.service.sprint.Sprint@4ec59d6[completeDate=<null>,endDate=2020-07-06T16:16:00.000Z,goal=Code Complete for Agent 4.0, and something else,id=190,name=Gold Sprint 2,rapidViewId=75,sequence=190,startDate=2020-06-22T16:16:28.226Z,state=ACTIVE]`, "190"},
	}

	for _, c := range cases {
		kv, err := parseSprintIntoKV(c.In)
		if err != nil {
			t.Error("errored: %w", err)
		}
		got := kv[c.Label]
		if got != c.Want {
			t.Errorf("case '%v' wanted `%v` got `%v`", c.Label, c.Want, got)
		}
	}
}

func TestAdjustRenderedHTML(t *testing.T) {
	cases := []struct {
		Label      string
		In         string
		WebsiteURL string
		Want       string
	}{
		{
			Label:      "empty",
			In:         "",
			WebsiteURL: "",
			Want:       "",
		},
		{
			Label:      "basic string",
			In:         "<p>example</p>",
			WebsiteURL: "",
			Want:       `<div class="source-jira"><p>example</p></div>`,
		},
		{
			Label:      "fixing image links",
			In:         "<p>something something</p>\n\n<p> <span class=\"image-wrap\" style=\"\"><a id=\"11887_thumb\" href=\"/secure/attachment/11887/11887_suggested+work+mentors.png\" title=\"suggested work mentors.png\" file-preview-type=\"image\" file-preview-id=\"11887\" file-preview-title=\"suggested work mentors.png\"><jira-attachment-thumbnail url=\"https://example.com/secure/thumbnail/11887/suggested+work+mentors.png?default=false\" jira-url=\"https://example.com/secure/thumbnail/11887/suggested+work+mentors.png\" filename=\"suggested work mentors.png\"><img src=\"https://example.com/secure/thumbnail/11887/suggested+work+mentors.png\" data-attachment-name=\"suggested work mentors.png\" data-attachment-type=\"thumbnail\" data-media-services-id=\"0f465f89-3c54-46d7-a311-cce52b438b85\" data-media-services-type=\"file\" style=\"border: 0px solid black\" /></jira-attachment-thumbnail></a></span> </p>",
			WebsiteURL: "",
			Want: `<div class="source-jira"><p>something something</p>

<p> <span class="image-wrap" style=""><img src="https://example.com/secure/thumbnail/11887/suggested+work+mentors.png" /></span> </p></div>`,
		},
		{
			Label:      "fixing emoticons",
			In:         "<p><img class=\"emoticon\" src=\"/images/icons/emoticons/smile.png\" height=\"16\" width=\"16\" align=\"absmiddle\" alt=\"\" border=\"0\"/> xzs</p>",
			WebsiteURL: "https://example.com",
			Want:       `<div class="source-jira"><p><img class="emoticon" src="https://example.com/images/icons/emoticons/smile.png" height="16" width="16" align="absmiddle" alt="" border="0"/> xzs</p></div>`,
		},
	}
	for _, c := range cases {
		got := adjustRenderedHTML(c.WebsiteURL, c.In)
		if got != c.Want {
			t.Errorf("failed case\n%v\nwant\n%v\ngot\n%v", c.Label, c.Want, got)
		}
	}
}

func TestCreateChangeLog(t *testing.T) {
	assert := assert.New(t)
	ts := time.Now()
	item := changeLogItem{
		Field:      "status",
		From:       "1",
		FromString: "To Do",
		To:         "6",
		ToString:   "Closed",
	}
	c := createChangeLog("1234", "1", "robin", ts, 1, item)
	assert.NotNil(c)
	assert.Equal(sdk.WorkIssueChangeLogFieldStatus, c.Field)
	assert.Equal("Closed", c.To)
	assert.Equal("To Do", c.From)
}
