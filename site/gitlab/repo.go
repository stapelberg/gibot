// Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package gitlab

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/mvdan/gibot/site"

	client "github.com/xanzy/go-gitlab"
)

type Repo struct {
	Name     string
	Path     string
	IssuesRe *regexp.Regexp
	PullsRe  *regexp.Regexp
	Client   client.Client
}

func NewRepo(r *site.Repo) *Repo {
	issuesRe := regexp.MustCompile(`(` + strings.Join(r.Aliases, "|") + `)#([1-9][0-9]*)`)
	issuesRe.Longest()
	pullsRe := regexp.MustCompile(`(` + strings.Join(r.Aliases, "|") + `)!([1-9][0-9]*)`)
	pullsRe.Longest()
	return &Repo{
		Name:     r.Name,
		Path:     r.Path,
		IssuesRe: issuesRe,
		PullsRe:  pullsRe,
		Client:   *client.NewClient(nil, r.Token),
	}
}

func (r *Repo) issueURL(id string) string {
	return fmt.Sprintf("https://gitlab.com/%s/issues/%s", r.Path, id)
}

func (r *Repo) getIssue(id string) (*client.Issue, error) {
	n, err := strconv.Atoi(id)
	if err != nil {
		return nil, err
	}
	issues, _, err := r.Client.Issues.ListProjectIssues(r.Path,
		&client.ListProjectIssuesOptions{IID: n})
	if err != nil {
		return nil, err
	}
	if len(issues) < 1 {
		return nil, fmt.Errorf("Not found")
	}
	return &issues[0], nil
}

func (r *Repo) IssueInfo(id string) (string, error) {
	issue, err := r.getIssue(id)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("#%s: %s - %s", id, issue.Title, r.issueURL(id)), nil
}

func (r *Repo) getMergeRequest(id string) (*client.MergeRequest, error) {
	n, err := strconv.Atoi(id)
	if err != nil {
		return nil, err
	}
	merges, _, err := r.Client.MergeRequests.ListMergeRequests(r.Path,
		&client.ListMergeRequestsOptions{IID: n})
	if err != nil {
		return nil, err
	}
	if len(merges) < 1 {
		return nil, fmt.Errorf("Not found")
	}
	return &merges[0], nil
}

func (r *Repo) mergeURL(id string) string {
	return fmt.Sprintf("https://gitlab.com/%s/merge_requests/%s", r.Path, id)
}

func (r *Repo) PullInfo(id string) (string, error) {
	merge, err := r.getMergeRequest(id)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("#%s: %s - %s", id, merge.Title, r.mergeURL(id)), nil
}
