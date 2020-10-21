package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/RichardKnop/machinery/v1/tasks"
	uuid "github.com/satori/go.uuid"
	activitypub "github.com/yukimochi/Activity-Relay/ActivityPub"
	state "github.com/yukimochi/Activity-Relay/State"
)

func pushRegistorJob(inboxURL string, body []byte) {
	job := &tasks.Signature{
		Name:       "registor",
		RetryCount: 25,
		Args: []tasks.Arg{
			{
				Name:  "inboxURL",
				Type:  "string",
				Value: inboxURL,
			},
			{
				Name:  "body",
				Type:  "string",
				Value: string(body),
			},
		},
	}
	_, err := machineryServer.SendTask(job)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func createFollowRequestResponse(domain string, response string) error {
	data, err := relayState.RedisClient.HGetAll("relay:pending:" + domain).Result()
	if err != nil {
		return err
	}
	activity := activitypub.Activity{
		Context: []string{"https://www.w3.org/ns/activitystreams", "https://w3id.org/security/v1"},
		ID:      data["activity_id"],
		Actor:   data["actor"],
		Type:    data["type"],
		Object:  data["object"],
	}

	resp := activity.GenerateResponse(hostname, response)
	jsonData, err := json.Marshal(&resp)
	if err != nil {
		return err
	}
	pushRegistorJob(data["inbox_url"], jsonData)
	relayState.RedisClient.Del("relay:pending:" + domain)
	if response == "Accept" {
		relayState.AddSubscription(state.Subscription{
			Domain:     domain,
			InboxURL:   data["inbox_url"],
			ActivityID: data["activity_id"],
			ActorID:    data["actor"],
		})
	}

	return nil
}

func createUpdateActorActivity(subscription state.Subscription) error {
	activity := activitypub.Activity{
		Context: []string{"https://www.w3.org/ns/activitystreams"},
		ID:      hostname.String() + "/activities/" + uuid.NewV4().String(),
		Actor:   hostname.String() + "/actor",
		Type:    "Update",
		To:      []string{"https://www.w3.org/ns/activitystreams#Public"},
		Object:  Actor,
	}

	jsonData, err := json.Marshal(&activity)
	if err != nil {
		return err
	}
	pushRegistorJob(subscription.InboxURL, jsonData)

	return nil
}

func listFollows() ([]string, error) {
	var domains []string
	follows, err := relayState.RedisClient.Keys("relay:pending:*").Result()
	if err != nil {
		return domains, err
	}
	for _, follow := range follows {
		domains = append(domains, strings.Replace(follow, "relay:pending:", "", 1))
	}
	return domains, nil
}

func acceptFollow(domain string) error {
	var err error
	var domains []string
	follows, err := relayState.RedisClient.Keys("relay:pending:*").Result()
	if err != nil {
		return err
	}
	for _, follow := range follows {
		domains = append(domains, strings.Replace(follow, "relay:pending:", "", 1))
	}
	if contains(domains, domain) {
		createFollowRequestResponse(domain, "Accept")
		return nil
	} else {
		return errors.New("Invalid domain [" + domain + "] given")
	}

	return nil
}

func rejectFollow(domain string) error {
	var err error
	var domains []string
	follows, err := relayState.RedisClient.Keys("relay:pending:*").Result()
	if err != nil {
		return err
	}
	for _, follow := range follows {
		domains = append(domains, strings.Replace(follow, "relay:pending:", "", 1))
	}
	for _, request := range domains {
		if domain == request {
			createFollowRequestResponse(domain, "Reject")
			return nil
		}
	}
	return errors.New("Invalid domain [" + domain + "] given")
}

func updateActor() error {
	for _, subscription := range relayState.Subscriptions {
		err := createUpdateActorActivity(subscription)
		if err != nil {
			return errors.New("Failed Update Actor for " + subscription.Domain)
		}
	}
	return nil
}
