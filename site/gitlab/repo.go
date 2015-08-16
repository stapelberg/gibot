// Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package gitlab

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/mvdan/gibot/site"

	client "github.com/xanzy/go-gitlab"
)

type Repo struct {
	Name     string
	Prefix   string
	Path     string
	IssueRe  *regexp.Regexp
	PullRe   *regexp.Regexp
	CommitRe *regexp.Regexp
	Client   client.Client
}

func NewRepo(r *site.Repo) *Repo {
	issueRe := regexp.MustCompile(`(` + strings.Join(r.Aliases, "|") + `)#([1-9][0-9]*)`)
	issueRe.Longest()
	pullRe := regexp.MustCompile(`(` + strings.Join(r.Aliases, "|") + `)!([1-9][0-9]*)`)
	pullRe.Longest()
	commitRe := regexp.MustCompile(`[0-9a-f]{6,20}`)
	commitRe.Longest()
	return &Repo{
		Name:     r.Name,
		Prefix:   r.Prefix,
		Path:     r.Path,
		IssueRe:  issueRe,
		PullRe:   pullRe,
		CommitRe: commitRe,
		Client:   *client.NewClient(nil, r.Token),
	}
}

func (r *Repo) GetUser(id int) (*client.User, error) {
	user, _, err := r.Client.Users.GetUser(id)
	return user, err
}

func (r *Repo) IssueURL(id int) string {
	return fmt.Sprintf("%s/%s/issues/%d", r.Prefix, r.Path, id)
}

func (r *Repo) GetIssue(id int) (*client.Issue, error) {
	issues, _, err := r.Client.Issues.ListProjectIssues(r.Path,
		&client.ListProjectIssuesOptions{IID: id})
	if err != nil {
		return nil, err
	}
	if len(issues) < 1 {
		return nil, fmt.Errorf("Not found")
	}
	return &issues[0], nil
}

func (r *Repo) IssueInfo(id int) (string, error) {
	issue, err := r.GetIssue(id)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("#%d: %s - %s", id, issue.Title, r.IssueURL(id)), nil
}

func (r *Repo) GetMergeRequest(id int) (*client.MergeRequest, error) {
	merges, _, err := r.Client.MergeRequests.ListMergeRequests(r.Path,
		&client.ListMergeRequestsOptions{IID: id})
	if err != nil {
		return nil, err
	}
	if len(merges) < 1 {
		return nil, fmt.Errorf("Not found")
	}
	return &merges[0], nil
}

func (r *Repo) MergeURL(id int) string {
	return fmt.Sprintf("%s/%s/merge_requests/%d", r.Prefix, r.Path, id)
}

func (r *Repo) PullInfo(id int) (string, error) {
	merge, err := r.GetMergeRequest(id)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("#%d: %s - %s", id, merge.Title, r.MergeURL(id)), nil
}

func (r *Repo) GetCommit(sha string) (*client.Commit, error) {
	commit, _, err := r.Client.Commits.GetCommit(r.Path, sha)
	return commit, err
}

func shortCommit(sha string) string {
	if len(sha) > 6 {
		return sha[:6]
	}
	return sha
}

func (r *Repo) CommitURL(sha string) string {
	sha = shortCommit(sha)
	return fmt.Sprintf("%s/%s/commit/%s", r.Prefix, r.Path, sha)
}

func (r *Repo) CommitInfo(sha string) (string, error) {
	commit, err := r.GetCommit(sha)
	if err != nil {
		return "", err
	}
	short := commit.ShortID
	return fmt.Sprintf("%s: %s - %s", short, commit.Title, r.CommitURL(short)), nil
}

func (r *Repo) CompareURL(before, after string) string {
	before = shortCommit(before)
	after = shortCommit(after)
	return fmt.Sprintf("%s/%s/compare/%s...%s", r.Prefix, r.Path, before, after)
}
