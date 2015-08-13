// Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package gitlab

import (
	"fmt"
	"regexp"
	"strings"
)

type Repo struct {
	Name     string
	URL      string
	IssuesRe *regexp.Regexp
	PullsRe  *regexp.Regexp
}

func NewRepo(name string, url string, aliases ...string) Repo {
	issuesRe := regexp.MustCompile(`(` + strings.Join(aliases, "|") + `)#([1-9][0-9]*)`)
	issuesRe.Longest()
	pullsRe := regexp.MustCompile(`(` + strings.Join(aliases, "|") + `)!([1-9][0-9]*)`)
	pullsRe.Longest()
	return Repo{
		Name:     name,
		URL:      url,
		IssuesRe: issuesRe,
		PullsRe:  pullsRe,
	}
}

func (r Repo) IssueURL(id string) string {
	return fmt.Sprintf("%s/issues/%s", r.URL, id)
}

func (r Repo) PullURL(id string) string {
	return fmt.Sprintf("%s/merge_requests/%s", r.URL, id)
}
