// Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"

	"github.com/mvdan/gibot/site/gitlab"
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

func toInt(v interface{}) int {
	i, ok := v.(float64)
	if !ok {
		return 0
	}
	return int(i)
}

func toStr(v interface{}) string {
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

func toSlice(v interface{}) []interface{} {
	l, ok := v.([]interface{})
	if !ok {
		return []interface{}{}
	}
	return l
}

func toMap(v interface{}) map[string]interface{} {
	m, ok := v.(map[string]interface{})
	if !ok {
		return map[string]interface{}{}
	}
	return m
}

func gitlabHandler(reponame string) func(http.ResponseWriter, *http.Request) {
	repo, e := repos[reponame]
	if !e {
		panic("unknown repo")
	}
	return func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)
		m := make(map[string]interface{})
		if err := decoder.Decode(&m); err != nil {
			log.Printf("Error decoding webhook data: %v", err)
			return
		}
		kind := toStr(m["object_kind"])
		switch kind {
		case "push":
			onPush(repo, m)
		case "issue":
			onIssue(repo, m)
		case "merge_request":
			onMergeRequest(repo, m)
		default:
			log.Printf("Webhook event we don't handle: %s", kind)
			return
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

func onPush(r *gitlab.Repo, m map[string]interface{}) {
	userId := toInt(m["user_id"])
	user, err := r.GetUser(userId)
	if err != nil {
		log.Printf("Unknown user: %v", err)
		return
	}
	username := user.Username
	count := toInt(m["total_commits_count"])
	var howMany string
	if count > 1 {
		howMany = fmt.Sprintf("%d commits", count)
	} else {
		howMany = fmt.Sprintf("%d commit", count)
	}
	branch := getBranch(toStr(m["ref"]))
	if branch == "" {
		return
	}
	before := toStr(m["before"])
	after := toStr(m["after"])
	url := r.CompareURL(before, after)
	message := fmt.Sprintf("%s pushed %s to %s - %s", username, howMany, branch, url)
	sendNoticeToAll(r.Name, message)
}

func onIssue(r *gitlab.Repo, m map[string]interface{}) {
	user := toMap(m["user"])
	username := toStr(user["username"])
	attrs := toMap(m["object_attributes"])
	iid := toInt(attrs["iid"])
	title := gitlab.ShortTitle(toStr(attrs["title"]))
	url := toStr(attrs["url"])
	action := toStr(attrs["action"])
	var message string
	switch action {
	case "open":
		message = fmt.Sprintf("%s opened #%d: %s - %s", username, iid, title, url)
	case "close":
		message = fmt.Sprintf("%s closed #%d: %s - %s", username, iid, title, url)
	case "reopen":
		message = fmt.Sprintf("%s reopened #%d: %s - %s", username, iid, title, url)
	default:
		log.Printf("Issue action we don't handle: %s", action)
		return
	}
	sendNoticeToAll(r.Name, message)
}

func onMergeRequest(r *gitlab.Repo, m map[string]interface{}) {
	user := toMap(m["user"])
	username := toStr(user["username"])
	attrs := toMap(m["object_attributes"])
	iid := toInt(attrs["iid"])
	title := gitlab.ShortTitle(toStr(attrs["title"]))
	url := toStr(attrs["url"])
	action := toStr(attrs["action"])
	var message string
	switch action {
	case "open":
		message = fmt.Sprintf("%s opened !%d: %s - %s", username, iid, title, url)
	case "close":
		message = fmt.Sprintf("%s closed !%d: %s - %s", username, iid, title, url)
	case "reopen":
		message = fmt.Sprintf("%s reopened !%d: %s - %s", username, iid, title, url)
	default:
		log.Printf("Merge Request action we don't handle: %s", action)
		return
	}
	sendNoticeToAll(r.Name, message)
}
