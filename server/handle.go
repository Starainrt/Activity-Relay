package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"

	"b612.me/starlog"
	"b612.me/starmap"
	"github.com/RichardKnop/machinery/v1/tasks"
	activitypub "github.com/starainrt/Activity-Relay/ActivityPub"
	"github.com/starainrt/Activity-Relay/conf"
)

func handleWebfinger(writer http.ResponseWriter, request *http.Request) {
	resource := request.URL.Query()["resource"]
	if request.Method != "GET" || len(resource) == 0 {
		writer.WriteHeader(400)
		writer.Write(nil)
	} else {
		request := resource[0]
		if request == WebfingerResource.Subject {
			wfresource, err := json.Marshal(&WebfingerResource)
			if err != nil {
				panic(err)
			}
			writer.Header().Add("Content-Type", "application/json")
			writer.WriteHeader(200)
			writer.Write(wfresource)
		} else {
			writer.WriteHeader(404)
			writer.Write(nil)
		}
	}
}

func handleNodeinfoLink(writer http.ResponseWriter, request *http.Request) {
	if request.Method != "GET" {
		writer.WriteHeader(400)
		writer.Write(nil)
	} else {
		linksresource, err := json.Marshal(&Nodeinfo.NodeinfoLinks)
		if err != nil {
			panic(err)
		}
		writer.Header().Add("Content-Type", "application/json")
		writer.WriteHeader(200)
		writer.Write(linksresource)
	}
}

func handleNodeinfo(writer http.ResponseWriter, request *http.Request) {
	if request.Method != "GET" {
		writer.WriteHeader(400)
		writer.Write(nil)
	} else {
		userCount := len(relayState.Subscriptions)
		Nodeinfo.Nodeinfo.Usage.Users.Total = userCount
		Nodeinfo.Nodeinfo.Usage.Users.ActiveMonth = userCount
		Nodeinfo.Nodeinfo.Usage.Users.ActiveHalfyear = userCount
		linksresource, err := json.Marshal(&Nodeinfo.Nodeinfo)
		if err != nil {
			panic(err)
		}
		writer.Header().Add("Content-Type", "application/json")
		writer.WriteHeader(200)
		writer.Write(linksresource)
	}
}

func handleActor(writer http.ResponseWriter, request *http.Request) {
	if request.Method == "GET" {
		actor, err := json.Marshal(&Actor)
		if err != nil {
			panic(err)
		}
		writer.Header().Add("Content-Type", "application/activity+json")
		writer.WriteHeader(200)
		writer.Write(actor)
	} else {
		writer.WriteHeader(400)
		writer.Write(nil)
	}
}

func contains(entries interface{}, finder string) bool {
	switch entry := entries.(type) {
	case string:
		return entry == finder
	case []string:
		for i := 0; i < len(entry); i++ {
			if entry[i] == finder {
				return true
			}
		}
		return false
	case []conf.Subscription:
		for i := 0; i < len(entry); i++ {
			if entry[i].Domain == finder {
				return true
			}
		}
		return false
	}
	return false
}

func pushRelayJob(sourceInbox string, body []byte) {
	for _, domain := range relayState.Subscriptions {
		if sourceInbox != domain.Domain {
			starlog.Debugf("Now Subscripted Domain：%s \n", domain.Domain)
			job := &tasks.Signature{
				Name:       "relay",
				RetryCount: 0,
				Args: []tasks.Arg{
					{
						Name:  "inboxURL",
						Type:  "string",
						Value: domain.InboxURL,
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
	}
}

func pushRegistorJob(inboxURL string, body []byte) {
	job := &tasks.Signature{
		Name:       "registor",
		RetryCount: 2,
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

func followAcceptable(activity *activitypub.Activity, actor *activitypub.Actor) error {
	/*
		if contains(activity.Object, "https://www.w3.org/ns/activitystreams#Public") {
			return nil
		} else {
			return errors.New("Follow only allowed for https://www.w3.org/ns/activitystreams#Public")
		}
	*/
	return nil
}

func unFollowAcceptable(activity *activitypub.Activity, actor *activitypub.Actor) error {
	/*
		if contains(activity.Object, "https://www.w3.org/ns/activitystreams#Public") {
			return nil
		} else {
			return errors.New("Unfollow only allowed for https://www.w3.org/ns/activitystreams#Public")
		}
	*/
	return nil
}

func suitableFollow(activity *activitypub.Activity, actor *activitypub.Actor) bool {
	domain, _ := url.Parse(activity.Actor)
	if contains(relayState.BlockedDomains, domain.Host) {
		return false
	}
	return true
}

func relayAcceptable(activity *activitypub.Activity, actor *activitypub.Actor) error {
	/*
		if !contains(activity.To, "https://www.w3.org/ns/activitystreams#Public") && !contains(activity.Cc, "https://www.w3.org/ns/activitystreams#Public") {
			return errors.New("Activity should contain https://www.w3.org/ns/activitystreams#Public as receiver")
		}
	*/
	domain, _ := url.Parse(activity.Actor)
	if contains(relayState.Subscriptions, domain.Host) {
		return nil
	}
	return errors.New("To use the relay service, Subscribe me in advance")
}

func suitableRelay(activity *activitypub.Activity, actor *activitypub.Actor) bool {
	domain, _ := url.Parse(activity.Actor)
	if contains(relayState.LimitedDomains, domain.Host) {
		return false
	}
	if relayState.RelayConfig.BlockService && actor.Type != "Person" {
		return false
	}
	return true
}

func RemoteIp(req *http.Request, header string) string {
	remoteAddr, _, _ := net.SplitHostPort(req.RemoteAddr)
	if header != "origin" {
		if ip := req.Header.Get("Remote_addr"); ip != "" {
			return ip
		}
	}
	if remoteAddr == "::1" {
		remoteAddr = "127.0.0.1"
	}
	return remoteAddr
}

func handleInbox(writer http.ResponseWriter, request *http.Request, activityDecoder func(*http.Request) (*activitypub.Activity, *activitypub.Actor, []byte, error)) {
	log := starlog.Std.NewFlag()
	defer log.Close()
	cfg := starmap.MustGet("config").(conf.RelayConfig)
	switch request.Method {
	case "POST":
		activity, actor, body, err := activityDecoder(request)
		if err != nil {
			log.Errorln("Cannot Decode New Request !", err)
			writer.WriteHeader(400)
			writer.Write(nil)
		} else {
			domain, _ := url.Parse(activity.Actor)
			switch activity.Type {
			case "Follow":
				log.Noticeln("Recv New Follow Request From", domain.Host)
				err = followAcceptable(activity, actor)
				if err != nil {
					resp := activity.GenerateResponse(hostURL, "Reject")
					jsonData, _ := json.Marshal(&resp)
					go pushRegistorJob(actor.Endpoints.SharedInbox, jsonData)
					log.Noticeln("Reject Follow Request Due to Bloctlist: ", err.Error(), activity.Actor)
					writer.WriteHeader(202)
					writer.Write(nil)
				} else {
					if suitableFollow(activity, actor) {
						if relayState.RelayConfig.ManuallyAccept {
							relayState.RedisClient.HMSet("relay:pending:"+domain.Host, map[string]interface{}{
								"inbox_url":   actor.Endpoints.SharedInbox,
								"activity_id": activity.ID,
								"type":        "Follow",
								"actor":       actor.ID,
								"object":      activity.Object.(string),
							})
							log.Infoln("Pending Follow Request : ", activity.Actor)
						} else {
							// hook
							shoudAccept := CheckShouldAccept(RemoteIp(request, cfg.IPSource), domain, activity, actor)
							if shoudAccept == 1 {
								resp := activity.GenerateResponse(hostURL, "Reject")
								jsonData, _ := json.Marshal(&resp)
								go pushRegistorJob(actor.Endpoints.SharedInbox, jsonData)
								writer.WriteHeader(202)
								writer.Write(nil)
								return
							} else if shoudAccept == 2 {
								log.Noticeln("Auto Pending : ", activity.Actor)
								writer.WriteHeader(202)
								writer.Write(nil)
								return
							}
							// hook end
							resp := activity.GenerateResponse(hostURL, "Accept")
							jsonData, _ := json.Marshal(&resp)
							go pushRegistorJob(actor.Endpoints.SharedInbox, jsonData)
							relayState.AddSubscription(conf.Subscription{
								Domain:     domain.Host,
								InboxURL:   actor.Endpoints.SharedInbox,
								ActivityID: activity.ID,
								ActorID:    actor.ID,
							})
							log.Infof("Accept Follow Request : %s Inbox: %s\n", activity.Actor, actor.Endpoints.SharedInbox)
							//pelor
							info := GetInstancesInfo(domain.Host)
							log.Debugf("Instances %s Info: %v\n", domain.Host, info)
							if info.Software != "mastodon" {
								fb := activity.GeneratebackRequest(hostURL, "Follow")
								fbjsonData, _ := json.Marshal(&fb)
								go pushRegistorJob(actor.Endpoints.SharedInbox, fbjsonData)
								log.Infoln("Send Follow Back Request : ", activity.Actor)
							}
						}
					} else {
						resp := activity.GenerateResponse(hostURL, "Reject")
						jsonData, _ := json.Marshal(&resp)
						go pushRegistorJob(actor.Endpoints.SharedInbox, jsonData)
						log.Infoln("Reject Follow Request : ", activity.Actor)
					}

					writer.WriteHeader(202)
					writer.Write(nil)
				}
			case "Undo":
				nestedActivity, _ := activity.NestedActivity()
				if nestedActivity.Type == "Follow" && nestedActivity.Actor == activity.Actor {
					err = unFollowAcceptable(nestedActivity, actor)
					if err != nil {
						log.Infoln("Reject Unfollow Request due to blacklist: ", err.Error())
						writer.WriteHeader(400)
						writer.Write([]byte(err.Error()))
					} else {
						info := GetInstancesInfo(domain.Host)
						relayState.DelSubscription(domain.Host)
						log.Infoln("Accept Unfollow Request : %+v", activity.Actor)
						if info.Software != "mastodon" {
							fb := activity.GeneratebackRequest(hostURL, "Undo")
							fbjsonData, _ := json.Marshal(&fb)
							go pushRegistorJob(actor.Endpoints.SharedInbox, fbjsonData)
							DelInstancesInfo(domain.Host)
							log.Infoln("Send Undo Back Request : ", activity.Actor)
						}
						writer.WriteHeader(202)
						writer.Write(nil)
					}
				} else {
					err = relayAcceptable(activity, actor)
					if err != nil {
						log.Errorln("Request Reject!", activity.Type)
						writer.WriteHeader(400)
						writer.Write([]byte(err.Error()))
					} else {
						domain, _ := url.Parse(activity.Actor)
						go pushRelayJob(domain.Host, body)
						log.Infoln("Accept Relay Status : ", activity.Actor)
						writer.WriteHeader(202)
						writer.Write(nil)
					}
				}
			case "Announce":
				err = relayAcceptable(activity, actor)
				if err != nil {
					log.Errorln("Request Reject!", activity.Type)
					writer.WriteHeader(400)
					writer.Write([]byte(err.Error()))
				} else {
					if suitableRelay(activity, actor) {
						resp := activity.GenerateAnnounce(hostURL)
						if value, ok := activity.Object.(string); ok {
							resp.Object = value
							jsonData, _ := json.Marshal(&resp)
							go pushRelayJob(domain.Host, jsonData)
							log.Infoln("Swapping Announce : ", activity.Actor)
						} else {
							log.Infoln("Skipping Relay Status : ", activity.Actor)
						}
					} else {
						log.Infoln("Skipping Relay Status : ", activity.Actor)
					}

					writer.WriteHeader(202)
					writer.Write(nil)
				}
			case "Create", "Update", "Delete", "Move":
				err = relayAcceptable(activity, actor)
				if err != nil {
					writer.WriteHeader(400)
					writer.Write([]byte(err.Error()))
					log.Errorln("Request Reject!", activity.Type)
				} else {
					if suitableRelay(activity, actor) {
						if relayState.RelayConfig.CreateAsAnnounce && activity.Type == "Create" {
							nestedObject, err := activity.NestedActivity()
							if err != nil {
								log.Infoln("Fail Assert activity : activity.Actor")
							}
							switch nestedObject.Type {
							case "Note":
								resp := nestedObject.GenerateAnnounce(hostURL)
								jsonData, _ := json.Marshal(&resp)
								go pushRelayJob(domain.Host, jsonData)
								log.Infoln("Accept Announce Note : ", activity.Actor)
							default:
								log.Infoln("Skipping Announce", nestedObject.Type, ": ", activity.Actor)
							}
						} else {
							go pushRelayJob(domain.Host, body)
							log.Infoln("Accept Relay Status : ", activity.Actor)
						}
					} else {
						log.Infoln("Skipping Relay Status : ", activity.Actor)
					}

					writer.WriteHeader(202)
					writer.Write(nil)
				}
			}
		}
	default:
		writer.WriteHeader(404)
		writer.Write(nil)
	}
}
