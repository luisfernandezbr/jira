package internal

import "github.com/pinpt/agent.next/sdk"

func (u user) ToModel(customerID string, integrationInstanceID string, websiteURL string) (*sdk.WorkUser, error) {
	theuser := &sdk.WorkUser{}
	theuser.CustomerID = customerID
	theuser.IntegrationInstanceID = sdk.StringPointer(integrationInstanceID)
	theuser.RefID = u.RefID()
	theuser.RefType = refType
	theuser.Name = u.DisplayName
	theuser.AvatarURL = sdk.StringPointer(u.Avatars.Large)
	theuser.Email = sdk.StringPointer(u.EmailAddress)
	theuser.ID = sdk.NewWorkUserID(customerID, theuser.RefID, refType)
	theuser.Member = u.Active
	theuser.Username = u.Name
	if u.Name != "" {
		v := sdk.NewWorkUserID(customerID, refType, u.Name)
		theuser.AssociatedRefID = &v
	}
	if u.AccountID != "" {
		// this is cloud
		theuser.URL = sdk.StringPointer(websiteURL + "/jira/people/" + u.AccountID)
	} else {
		// this is hosted
		// TODO: not sure this actually works, that's the url that links to the user profile,
		// but on our test hosted server it hangs forever when used in jira
		theuser.URL = sdk.StringPointer(websiteURL + "/secure/ViewProfile.jspa?name=" + u.Key)
	}
	return theuser, nil
}

// easyjson:skip
type userManager struct {
	users                 map[string]bool
	customerID            string
	websiteURL            string
	pipe                  sdk.Pipe
	stats                 *stats
	integrationInstanceID string
}

func (m *userManager) emit(user user) error {
	refid := user.RefID()
	if m.users[refid] {
		return nil
	}
	object, err := user.ToModel(m.customerID, m.websiteURL, m.integrationInstanceID)
	if err != nil {
		return err
	}
	if err := m.pipe.Write(object); err != nil {
		return nil
	}
	m.stats.incUser()
	m.users[refid] = true
	return nil
}

func newUserManager(customerID string, websiteURL string, pipe sdk.Pipe, stats *stats, integrationInstanceID string) *userManager {
	return &userManager{
		users:                 make(map[string]bool),
		customerID:            customerID,
		websiteURL:            websiteURL,
		pipe:                  pipe,
		stats:                 stats,
		integrationInstanceID: integrationInstanceID,
	}
}
