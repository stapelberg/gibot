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

const (
	listenAddr = ":9990"
	listenPath = "/webhooks/gitlab"
)

func webhookListen() {
	http.HandleFunc(listenPath, gitlabHandler)
	log.Printf("Receiving webhooks on %s", listenPath)
	log.Fatal(http.ListenAndServe(listenAddr, nil))
}

func gitlabHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	event := strings.TrimSpace(r.Header.Get("X-Gitlab-Event"))
	var err error
	switch event {
	case "Push Hook":
		err = onPush(r.Body)
	case "Issue Hook":
		err = onIssue(r.Body)
	case "Merge Request Hook":
		err = onMergeRequest(r.Body)
	default:
		log.Printf("Webhook event we don't handle: %s", event)
	}
	if err != nil {
		log.Print(err)
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

func getRepo(apiRepo *api.Repository) (*gitlab.Repo, error) {
	repo := repos[apiRepo.Homepage]
	if repo == nil {
		return nil, fmt.Errorf("unknown repo: %s", apiRepo.Homepage)
	}
	return repo, nil
}

func onPush(body io.Reader) error {
	var pe api.PushEvent
	if err := json.NewDecoder(body).Decode(&pe); err != nil {
		return fmt.Errorf("invalid push event body: %v", err)
	}
	repo, err := getRepo(pe.Repository)
	if err != nil {
		return err
	}
	user, err := repo.GetUser(pe.UserID)
	if err != nil {
		return fmt.Errorf("unknown user: %v", err)
	}
	branch := getBranch(pe.Ref)
	if branch == "" {
		return fmt.Errorf("no branch")
	}
	var message string
	if pe.TotalCommitsCount > 1 {
		url := repo.CompareURL(pe.Before, pe.After)
		message = fmt.Sprintf("%s pushed %d commits to %s - %s",
			user.Username, pe.TotalCommitsCount, branch, url)
	} else {
		if len(pe.Commits) == 0 {
			return fmt.Errorf("empty commits")
		}
		commit := pe.Commits[0]
		// Message here means Title, how useful.
		title := gitlab.ShortTitle(commit.Message)
		short := gitlab.ShortCommit(commit.ID)
		message = fmt.Sprintf("%s pushed to %s: %s - %s",
			user.Username, branch, title, repo.CommitURL(short))
	}
	sendNotices(config.Feeds, repo.Name, message)
	return nil
}

func onIssue(body io.Reader) error {
	var ie api.IssueEvent
	if err := json.NewDecoder(body).Decode(&ie); err != nil {
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
	repo, err := getRepo(ie.Repository)
	if err != nil {
		return err
	}
	sendNotices(config.Feeds, repo.Name, message)
	return nil
}

func onMergeRequest(body io.Reader) error {
	var me api.MergeEvent
	if err := json.NewDecoder(body).Decode(&me); err != nil {
		return fmt.Errorf("invalid issue event body: %v", err)
	}
	attrs := me.ObjectAttributes
	title := gitlab.ShortTitle(attrs.Title)
	var message string
	switch attrs.Action {
	case "open":
		message = fmt.Sprintf("%s opened !%d: %s - %s",
			me.User.Username, attrs.Iid, title, attrs.URL)
	case "merge":
		message = fmt.Sprintf("%s merged !%d: %s - %s",
			me.User.Username, attrs.Iid, title, attrs.URL)
	case "close", "reopen", "update":
		return nil
	default:
		return fmt.Errorf("merge action we don't handle: %s", attrs.Action)
	}
	repo, err := getRepo(me.Repository)
	if err != nil {
		return err
	}
	sendNotices(config.Feeds, repo.Name, message)
	return nil
}
