// Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

func webhookListen() {
	http.HandleFunc("/webhooks/gitlab", gitlabHandler)

	log.Fatal(http.ListenAndServe(":9990", nil))
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

func toMap(v interface{}) map[string]interface{} {
	m, ok := v.(map[string]interface{})
	if !ok {
		return make(map[string]interface{})
	}
	return m
}

func gitlabHandler(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	m := make(map[string]interface{})
	if err := decoder.Decode(&m); err != nil {
		log.Printf("Error decoding webhook data: %v", err)
	}
	kind := toStr(m["object_kind"])
	switch kind {
	case "issue":
		onIssue(m)
	default:
		log.Printf("Webhook event we don't handle: %s", kind)
		return
	}
}

func onIssue(m map[string]interface{}) {
	user := toMap(m["user"])
	username := toStr(user["username"])
	attrs := toMap(m["object_attributes"])
	iid := toInt(attrs["iid"])
	title := toStr(attrs["title"])
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
	sendNoticeToAll("client", message)
}
