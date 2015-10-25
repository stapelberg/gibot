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

const (
	titleLength = 50
	shaLength   = 6
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
	commitRe := regexp.MustCompile(`\b[0-9a-f]{6,20}\b`)
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

func firstLine(s string) string {
	i := strings.IndexAny(s, "\n\r")
	if i == -1 {
		return s
	}
	return s[:i]
}

func ShortTitle(message string) string {
	title := firstLine(message)
	title = strings.TrimSpace(title)
	if len(title) > titleLength {
		return fmt.Sprintf("%s…", title[:titleLength-1])
	}
	return title
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
	return issues[0], nil
}

func (r *Repo) IssueInfo(id int) (string, error) {
	issue, err := r.GetIssue(id)
	if err != nil {
		return "", err
	}
	title := ShortTitle(issue.Title)
	return fmt.Sprintf("#%d: %s - %s", id, title, r.IssueURL(id)), nil
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
	return merges[0], nil
}

func (r *Repo) MergeURL(id int) string {
	return fmt.Sprintf("%s/%s/merge_requests/%d", r.Prefix, r.Path, id)
}

func (r *Repo) PullInfo(id int) (string, error) {
	merge, err := r.GetMergeRequest(id)
	if err != nil {
		return "", err
	}
	title := ShortTitle(merge.Title)
	return fmt.Sprintf("#%d: %s - %s", id, title, r.MergeURL(id)), nil
}

func (r *Repo) GetCommit(sha string) (*client.Commit, error) {
	commit, _, err := r.Client.Commits.GetCommit(r.Path, sha)
	return commit, err
}

func ShortCommit(sha string) string {
	if len(sha) > shaLength {
		return sha[:shaLength]
	}
	return sha
}

func (r *Repo) CommitURL(sha string) string {
	sha = ShortCommit(sha)
	return fmt.Sprintf("%s/%s/commit/%s", r.Prefix, r.Path, sha)
}

func (r *Repo) CommitInfo(sha string) (string, error) {
	commit, err := r.GetCommit(sha)
	if err != nil {
		return "", err
	}
	short := ShortCommit(sha)
	title := ShortTitle(commit.Title)
	return fmt.Sprintf("%s: %s - %s", short, title, r.CommitURL(short)), nil
}

func (r *Repo) CompareURL(before, after string) string {
	before = ShortCommit(before)
	after = ShortCommit(after)
	return fmt.Sprintf("%s/%s/compare/%s...%s", r.Prefix, r.Path, before, after)
}
