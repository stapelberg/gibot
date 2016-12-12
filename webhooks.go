// Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/mvdan/gibot/site/gitlab"

	api "github.com/xanzy/go-gitlab"
)

const listenAddr = ":9990"

func webhookListen() {
	for _, repo := range repos {
		listenRepo(repo)
	}

	log.Fatal(http.ListenAndServe(listenAddr, nil))
}

func listenRepo(repo *gitlab.Repo) {
	path := fmt.Sprintf("/webhooks/gitlab/%s", repo.Name)
	http.HandleFunc(path, gitlabHandler(repo.Name))
	log.Printf("Receiving webhooks for %s on %s%s", repo.Name, listenAddr, path)
}

func gitlabHandler(reponame string) func(http.ResponseWriter, *http.Request) {
	repo, e := repos[reponame]
	if !e {
		panic("unknown repo")
	}
	return func(w http.ResponseWriter, r *http.Request) {
		event := strings.TrimSpace(r.Header.Get("X-Gitlab-Event"))
		var err error
		switch event {
		case "Push Hook":
			err = onPush(repo, r.Body)
		case "Issue Hook":
			err = onIssue(repo, r.Body)
		case "Merge Request Hook":
			err = onMergeRequest(repo, r.Body)
		default:
			log.Printf("Webhook event we don't handle: %s", event)
		}
		if err != nil {
			log.Print(err)
		}
	}
}

var headBranch = regexp.MustCompile(`^refs/heads/(.*)$`)

func getBranch(ref string) string {
	if s := headBranch.FindStringSubmatch(ref); s != nil {
		return s[1]
	}
	log.Printf("Unknown branch ref format: %s", ref)
	return ""
}

func onPush(r *gitlab.Repo, rc io.ReadCloser) error {
	var pe api.PushEvent
	if err := json.NewDecoder(rc).Decode(&pe); err != nil {
		return fmt.Errorf("invalid push event body: %v", err)
	}
	user, err := r.GetUser(pe.UserID)
	if err != nil {
		return fmt.Errorf("unknown user: %v", err)
	}
	branch := getBranch(pe.Ref)
	if branch == "" {
		return fmt.Errorf("no branch")
	}
	var message string
	if pe.TotalCommitsCount > 1 {
		url := r.CompareURL(pe.Before, pe.After)
		message = fmt.Sprintf("%s pushed %d commits to %s - %s",
			user.Username, pe.TotalCommitsCount, branch, url)
	} else {
		if len(pe.Commits) == 0 {
			return fmt.Errorf("empty commits")
		}
		commit := pe.Commits[0]
		title := gitlab.ShortTitle(commit.Title)
		short := gitlab.ShortCommit(commit.ID)
		message = fmt.Sprintf("%s pushed to %s: %s - %s",
			user.Username, branch, title, r.CommitURL(short))
	}
	sendNotices(config.Feeds, r.Name, message)
	return nil
}

func onIssue(r *gitlab.Repo, rc io.ReadCloser) error {
	var ie api.IssueEvent
	if err := json.NewDecoder(rc).Decode(&ie); err != nil {
		return fmt.Errorf("invalid issue event body: %v", err)
	}
	attrs := ie.ObjectAttributes
	title := gitlab.ShortTitle(attrs.Title)
	var message string
	switch attrs.Action {
	case "open":
		message = fmt.Sprintf("%s opened #%d: %s - %s",
			ie.User.Username, attrs.Iid, title, attrs.URL)
	case "close":
		message = fmt.Sprintf("%s closed #%d: %s - %s",
			ie.User.Username, attrs.Iid, title, attrs.URL)
	case "reopen":
		message = fmt.Sprintf("%s reopened #%d: %s - %s",
			ie.User.Username, attrs.Iid, title, attrs.URL)
	case "update":
		return nil
	default:
		return fmt.Errorf("issue action we don't handle: %s", attrs.Action)
	}
	sendNotices(config.Feeds, r.Name, message)
	return nil
}

func onMergeRequest(r *gitlab.Repo, rc io.ReadCloser) error {
	var ie api.MergeEvent
	if err := json.NewDecoder(rc).Decode(&ie); err != nil {
		return fmt.Errorf("invalid issue event body: %v", err)
	}
	attrs := ie.ObjectAttributes
	title := gitlab.ShortTitle(attrs.Title)
	var message string
	switch attrs.Action {
	case "open":
		message = fmt.Sprintf("%s opened !%d: %s - %s",
			ie.User.Username, attrs.Iid, title, attrs.URL)
	case "merge":
		message = fmt.Sprintf("%s merged !%d: %s - %s",
			ie.User.Username, attrs.Iid, title, attrs.URL)
	case "close", "reopen", "update":
		return nil
	default:
		return fmt.Errorf("merge action we don't handle: %s", attrs.Action)
	}
	sendNotices(config.Feeds, r.Name, message)
	return nil
}
