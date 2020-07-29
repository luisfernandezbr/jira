package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type mockUserManager struct {
	users []user
}

func (m *mockUserManager) Emit(usr user) error {
	m.users = append(m.users, usr)
	return nil
}

func TestUserToModel(t *testing.T) {
	assert := assert.New(t)
	usr := user{
		Name:      "",
		Self:      "https://foo.bar/rest/api/2/user?accountId=151413:12abc",
		AccountID: "151413:12abc",
		Avatars: Avatars{
			XSmall: "https://xsmall.png",
			Small:  "https://small.png",
			Medium: "https://med.png",
			Large:  "https://large.png",
		},
		DisplayName: "testuser+1",
		Active:      true,
		Timezone:    "America/Los_Angeles",
	}
	output := usr.ToModel("1234", "1", "https://foo.bar")
	assert.EqualValues("1234", output.CustomerID)
	assert.EqualValues("1", *output.IntegrationInstanceID)
	assert.EqualValues("151413:12abc", output.RefID)
	assert.EqualValues("jira", output.RefType)
	assert.EqualValues("testuser+1", output.Name)
	assert.EqualValues("https://large.png", *output.AvatarURL)
	assert.Nil(output.Email)
	assert.EqualValues("7bfe75e72917c542", output.ID)
	assert.EqualValues("", output.Username)
	assert.EqualValues("https://foo.bar/jira/people/151413:12abc", *output.URL)
}
