package internal

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/pinpt/agent.next/sdk"
)

type comment struct {
	Self         string `json:"self"`
	ID           string `json:"id"`
	Author       user   `json:"author"`
	RenderedBody string `json:"renderedBody"`
	Created      string `json:"created"`
	Updated      string `json:"updated"`

	/**
		there is a visibility flag so we probably at some point want to consider
		bringing that into the model
		"visibility": {
	        "type": "role",
	        "value": "Administrators"
	      }*/
}

func (c comment) ToModel(customerID string, websiteURL string, userManager *userManager, projectID string, issueID string, issueKey string) (*sdk.WorkIssueComment, error) {
	if err := userManager.emit(c.Author); err != nil {
		return nil, err
	}
	comment := &sdk.WorkIssueComment{}
	comment.CustomerID = customerID
	comment.RefID = c.ID
	comment.RefType = refType
	comment.ProjectID = projectID
	comment.IssueID = issueID
	comment.Body = adjustRenderedHTML(websiteURL, c.RenderedBody)
	created, err := parseTime(c.Created)
	if err != nil {
		return nil, err
	}
	sdk.ConvertTimeToDateModel(created, &comment.CreatedDate)
	updated, err := parseTime(c.Updated)
	if err != nil {
		return nil, err
	}
	sdk.ConvertTimeToDateModel(updated, &comment.UpdatedDate)
	comment.UserRefID = c.Author.RefID()
	comment.URL = issueCommentURL(websiteURL, issueKey, c.ID)
	return comment, nil
}

type commentResult struct {
	Total    int       `json:"total"`
	Comments []comment `json:"comments"`
}

type issueForComment struct {
	ProjectID string
	IssueID   string
	IssueKey  string
}

type commentManager struct {
	logger      sdk.Logger
	export      sdk.Export
	pipe        sdk.Pipe
	integration *JiraIntegration
	authConfig  authConfig
	customerID  string
	userManager *userManager
	stats       *stats
	ch          chan issueForComment
	done        chan bool
	closing     bool
	mu          sync.RWMutex
}

func newCommentManager(logger sdk.Logger, pipe sdk.Pipe, integration *JiraIntegration, authConfig authConfig, customerID string, userManager *userManager, stats *stats) *commentManager {
	mgr := &commentManager{
		logger:      logger,
		ch:          make(chan issueForComment, 1),
		done:        make(chan bool, 1),
		pipe:        pipe,
		integration: integration,
		authConfig:  authConfig,
		customerID:  customerID,
		userManager: userManager,
		stats:       stats,
	}
	go mgr.run()
	return mgr
}

func (f *commentManager) run() {
	defer func() { f.done <- true }()
	for item := range f.ch {
		var startAt, count int
		for {
			url := fmt.Sprintf("/rest/api/3/issue/%s/comment?expand=renderedBody&startAt=%d&orderBy=-created", item.IssueKey, startAt)
			sdk.LogDebug(f.logger, "fetching comment for issue", "key", item.IssueKey, "url", url)
			cl := f.integration.httpmanager.New(sdk.JoinURL(f.authConfig.APIURL, url), nil)
			var res commentResult
			if resp, err := cl.Get(&res, f.authConfig.Middleware...); err != nil {
				if err := f.integration.checkForRateLimit(f.export, err, resp.Headers); err != nil {
					sdk.LogError(f.logger, "error fetching issue comment for issue", "key", item.IssueKey, "err", err)
					break
				}
				sdk.LogError(f.logger, "error fetching issue comment for issue", "key", item.IssueKey, "err", err)
				break
			}
			for _, c := range res.Comments {
				comment, err := c.ToModel(f.customerID, f.authConfig.WebsiteURL, f.userManager, item.ProjectID, item.IssueID, item.IssueKey)
				if err != nil {
					sdk.LogError(f.logger, "error convert issue comment model for issue", "key", item.IssueKey, "err", err)
					continue
				}
				if err := f.pipe.Write(comment); err != nil {
					sdk.LogError(f.logger, "error writing issue comment model for pipe for issue", "key", item.IssueKey, "err", err)
					continue
				}
				f.stats.incComment()
			}
			count += len(res.Comments)
			if count >= res.Total {
				break
			}
			startAt++
		}
		f.mu.RLock()
		closing := f.closing
		f.mu.RUnlock()
		if !closing {
			time.Sleep(time.Millisecond * time.Duration(rand.Int63n(200))) // randomly sleep a small amount so that this thread is throttled slower than the main one
		}
	}
}

func (f *commentManager) Close() error {
	f.mu.Lock()
	f.closing = true
	f.mu.Unlock()
	close(f.ch)
	<-f.done // wait for it to finish
	return nil
}

func (f *commentManager) add(projectID, issueID, issueKey string) error {
	f.ch <- issueForComment{projectID, issueID, issueKey}
	return nil
}
