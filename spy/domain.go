package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"strings"

	activitypub "github.com/yukimochi/Activity-Relay/ActivityPub"
	state "github.com/yukimochi/Activity-Relay/State"
)

func GetDomainList() ([]string, error) {
	var lists []string
	domains, err := redisClient.Keys("relay:subscription:*").Result()
	if err != nil {
		return lists, err
	}
	for _, domain := range domains {
		domainName := strings.Replace(domain, "relay:subscription:", "", 1)
		lists = append(lists, domainName)
	}
	return lists, nil
}

type GetNodeInfo struct {
	Rel  string `json:"rel"`
	Href string `json:"href"`
}

func CheckInstanceNum(domain string, byTotal bool) int {
	var nodes map[string][]GetNodeInfo
	var nodeinfo map[string]interface{}
	getNodeInfoUrl := "https://" + domain + "/.well-known/nodeinfo"
	resp, err := http.Get(getNodeInfoUrl)
	if err != nil {
		return -1
	}
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return -2
	}
	err = json.Unmarshal(data, &nodes)
	if err != nil {
		return -3
	}
	links, ok := nodes["links"]
	if !ok || len(links) < 1 {
		return -4
	}
	nodeInfoUrl := links[0].Href
	resp, err = http.Get(nodeInfoUrl)
	if err != nil {
		return -5
	}
	data, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return -6
	}
	err = json.Unmarshal(data, &nodeinfo)
	if err != nil {
		return -7
	}
	usage, ok := nodeinfo["usage"]
	if !ok {
		return -8
	}
	users := (usage.(map[string]interface{})["users"]).(map[string]interface{})
	if byTotal {
		return int(users["total"].(float64))
	}
	return int(users["activeMonth"].(float64))
}

func createUnfollowRequestResponse(subscription state.Subscription) error {
	activity := activitypub.Activity{
		Context: []string{"https://www.w3.org/ns/activitystreams", "https://w3id.org/security/v1"},
		ID:      subscription.ActivityID,
		Actor:   subscription.ActorID,
		Type:    "Follow",
		Object:  "https://www.w3.org/ns/activitystreams#Public",
	}

	resp := activity.GenerateResponse(hostname, "Reject")
	jsonData, _ := json.Marshal(&resp)
	pushRegistorJob(subscription.InboxURL, jsonData)

	return nil
}

func unfollowDomains(domain string) error {
	subscriptions := relayState.Subscriptions
	if contains(subscriptions, domain) {
		subscription := *relayState.SelectSubscription(domain)
		createUnfollowRequestResponse(subscription)
		relayState.DelSubscription(subscription.Domain)
		return nil
	}
	return errors.New("Invalid domain [" + domain + "] given")
}
