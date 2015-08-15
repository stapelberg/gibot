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

func (r *Repo) IssueURL(id string) string {
	return fmt.Sprintf("https://gitlab.com/%s/issues/%s", r.Path, id)
}

func (r *Repo) PullURL(id string) string {
	return fmt.Sprintf("https://gitlab.com/%s/merge_requests/%s", r.Path, id)
}
