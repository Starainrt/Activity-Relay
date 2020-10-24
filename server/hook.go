package server

import (
	"context"
	"encoding/json"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"b612.me/starmap"

	"b612.me/starlog"
	activitypub "github.com/starainrt/Activity-Relay/ActivityPub"
	"github.com/starainrt/Activity-Relay/conf"
)

//CheckShouldAccept 0 no 1 yes 2  pending
func CheckShouldAccept(ip string, domain *url.URL, activity *activitypub.Activity, actor *activitypub.Actor) int {
	log := starlog.Std.NewFlag()
	defer log.Close()
	cfg := starmap.MustGet("config").(conf.RelayConfig)
	info, err := conf.UpdateInstancesInfo(domain.Host)
	if err != nil {
		log.Errorf("Cannot Get %s Instances Info ,Error is %v\n", domain, err)
		relayState.RedisClient.HMSet("relay:autopending:"+domain.Host, map[string]interface{}{
			"inbox_url":   actor.Endpoints.SharedInbox,
			"activity_id": activity.ID,
			"type":        "Follow",
			"actor":       actor.ID,
			"object":      activity.Object.(string),
			"retry":       "0",
		})
		return 2
	}
	if cfg.MaxInstances < len(relayState.Subscriptions) {
		starlog.Noticeln("Cannot Accept,Limit Max Instances")
		return 1
	}
	log.Infof("Get Instances %s Info %+v\n", domain.Host, info)
	if !filterList(domain.Host) {
		log.Infof("Cannot Accept %s Due to Allow/Block List\n", domain.Host)
		return 1
	}
	var user int
	if cfg.UserByActive {
		user = info.ActiveUser
	} else {
		user = info.TotalUser
	}
	if (cfg.AllowMaxUser != 0 && cfg.AllowMaxUser < user) || cfg.AllowMinUser > user {
		log.Infof("Cannot Accept %s Due to User Count\n", domain.Host)
		return 1
	}
	jsonInfo, _ := json.Marshal(info)
	relayState.RedisClient.HSet("relay:info", domain.Host, string(jsonInfo)).Result()
	return 0
}

//CheckShouldKick 0 no 1 yes 2  pending
func CheckShouldKick(domain *url.URL, activity *activitypub.Activity, actor *activitypub.Actor) int {
	cfg := starmap.MustGet("config").(conf.RelayConfig)
	info, err := conf.UpdateInstancesInfo(domain.Host)
	if err != nil {
		starlog.Noticeln("Pending,Cannot Get Instance Info")
		return 2
	}
	if !filterList(domain.Host) {
		starlog.Noticeln("Cannot Accept Due to Allow/Reject Rule")
		return 1
	}
	var user int
	if cfg.UserByActive {
		user = info.ActiveUser
	} else {
		user = info.TotalUser
	}
	if (cfg.KickMaxUser != 0 && cfg.KickMaxUser > user) || cfg.KickMinUser < user {
		starlog.Noticeln("Cannot Accept,User too much")
		jsonInfo, _ := json.Marshal(info)
		relayState.RedisClient.HSet("relay:info", domain.Host, string(jsonInfo))
		return 1
	}
	return 0
}

func filterList(domain string) bool {
	allowlist := starmap.MustGet("allowlist").([]string)
	blocklist := starmap.MustGet("blocklist").([]string)
	starlog.Debugln(allowlist, len(allowlist))
	starlog.Debugln(blocklist, len(blocklist))
	for _, rep := range allowlist {
		should, err := regexp.MatchString(rep, domain)
		if err != nil {
			continue
		}
		if should {
			return true
		}
	}
	if len(allowlist) != 0 {
		return false
	}
	for _, rep := range blocklist {
		should, err := regexp.MatchString(rep, domain)
		if err != nil {
			continue
		}
		if should {
			return false
		}
	}
	return true
}

func updateAllInstancesInfo() {
	log := starlog.Std.NewFlag()
	defer log.Close()
	domains, err := relayState.RedisClient.Keys("relay:subscription:*").Result()
	if err != nil {
		return
	}
	cfg := starmap.MustGet("config").(conf.RelayConfig)
	for _, domain := range domains {
		domainName := strings.Replace(domain, "relay:subscription:", "", 1)
		oldInfo := GetInstancesInfo(domainName)
		log.Debugf("%v\n", oldInfo)
		info, err := conf.UpdateInstancesInfo(domainName)
		if err != nil {
			log.Errorf("Cannot Update Instance %s Info,Reason is %v\n", domainName, err)
			oldInfo.Retry++
			if oldInfo.Retry > cfg.KickAfter {
				log.Warningln("Instance %s Should Kick ,Cannot Get Info For %d Times\n", domainName, oldInfo.Retry)
				unfollowDomain(info)
				continue
			}
			jsonInfo, _ := json.Marshal(oldInfo)
			relayState.RedisClient.HSet("relay:info", domainName, string(jsonInfo)).Result()
			continue
		}
		log.Infof("Instance %s Info Updated %+v\n", domain, info)
		jsonInfo, _ := json.Marshal(info)
		relayState.RedisClient.HSet("relay:info", domainName, string(jsonInfo)).Result()
	}
	return
}

func UpdateInstancesProcess(stopCtx context.Context) {
	log := starlog.Std.NewFlag()
	defer log.Close()
	for {
		cfg := starmap.MustGet("config").(conf.RelayConfig)
		updateLimit := cfg.UpdateRate
		if updateLimit < 300 {
			updateLimit = 300
		}
		select {
		case <-stopCtx.Done():
			return
		case <-time.After(time.Second * time.Duration(updateLimit)):
			log.Noticeln("Now Updating Instances Info")
			updateAllInstancesInfo()
			log.Infoln("Instances Info Updated")
		}
	}
}

func GetInstancesInfo(domain string) conf.SubscriptionInfo {
	var info conf.SubscriptionInfo
	data, err := relayState.RedisClient.HGet("relay:info", domain).Result()
	if err != nil {
		starlog.Errorf("Cannot Get %s Info,%v\n", domain, err)
	}
	err = json.Unmarshal([]byte(data), &info)
	if err != nil {
		starlog.Errorf("Unmarshal %s Info Failed,%v\n", domain, err)
	}
	return info
}

func DelInstancesInfo(domain string) {
	relayState.RedisClient.HDel("relay:info", domain).Result()
}

func RecheckInstanceAcceptProcess(stopCtx context.Context) {
	log := starlog.Std.NewFlag()
	defer log.Close()
	for {
		select {
		case <-stopCtx.Done():
			return
		case <-time.After(time.Minute):
		}
		keys, err := relayState.RedisClient.Keys("relay:autopending:*").Result()
		if err != nil {
			continue
		}
		for _, key := range keys {
			domain := strings.Replace(key, "relay:autopending:", "", -1)
			domainInfo, err := relayState.RedisClient.HGetAll(key).Result()
			if err != nil {
				continue
			}
			retry, _ := strconv.Atoi(domainInfo["retry"])
			retry++
			info, err := conf.UpdateInstancesInfo(domain)
			if err != nil {
				log.Errorln("Cannot Got Auto Pending Instance Info", domain, err)
				if retry > 3 {
					log.Errorf("Cannot Got Auto Pending Instance %s Info for %d times,reject!\n", domain, retry)
					relayState.RedisClient.Del(key)
					DelInstancesInfo(domain)
					rejectDomain(info)
					log.Noticef("Instance %s Rejected!\n", domain)
				}
				continue
			}
			relayState.RedisClient.Del(key)
			jsonInfo, _ := json.Marshal(info)
			relayState.RedisClient.HSet("relay:info", domain, string(jsonInfo)).Result()
			log.Infof("Instance %s Accepted,Info Got!\n", domain)
			acceptDomain(info)
		}
	}
}

func KickInstancesProcess(stopCtx context.Context) {
	log := starlog.Std.NewFlag()
	defer log.Close()
	for {
		select {
		case <-stopCtx.Done():
			return
		case <-time.After(time.Second * 600):
		}
		cfg := starmap.MustGet("config").(conf.RelayConfig)
		keys, err := relayState.RedisClient.HKeys("relay:info").Result()
		if err != nil {
			continue
		}
		for _, domain := range keys {
			info := GetInstancesInfo(domain)
			if !filterList(info.Domain) {
				log.Warningf("Instcnce %s Kicked Due to Allow/Block Rules\n", info.Domain)
				unfollowDomain(info)
				continue
			}
			var user int
			if cfg.UserByActive {
				user = info.ActiveUser
			} else {
				user = info.TotalUser
			}
			if (cfg.KickMaxUser != 0 && cfg.KickMaxUser < user) || cfg.KickMinUser > user {
				log.Warningf("Instcnce %s Kicked Due to Number of People\n", info.Domain)
				unfollowDomain(info)
			}
		}

	}
}

func acceptDomain(info conf.SubscriptionInfo) error {

	data, err := relayState.RedisClient.HGetAll("relay:autopending:" + info.Domain).Result()
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
	resp := activity.GenerateResponse(hostURL, "Accept")
	jsonData, err := json.Marshal(&resp)
	if err != nil {
		return err
	}
	pushRegistorJob(data["inbox_url"], jsonData)
	relayState.RedisClient.Del("relay:autopending:" + info.Domain)
	relayState.AddSubscription(conf.Subscription{
		Domain:     info.Domain,
		InboxURL:   data["inbox_url"],
		ActivityID: data["activity_id"],
		ActorID:    data["actor"],
	})
	if info.Software != "mastodon" {
		fb := activity.GeneratebackRequest(hostURL, "Follow")
		fbjsonData, _ := json.Marshal(&fb)
		go pushRegistorJob(data["inbox_url"], fbjsonData)
		starlog.Infoln("Send Follow Back Request : ", activity.Actor)
	}
	return nil
}

func rejectDomain(info conf.SubscriptionInfo) error {
	data, err := relayState.RedisClient.HGetAll("relay:autopending:" + info.Domain).Result()
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
	resp := activity.GenerateResponse(hostURL, "Reject")
	jsonData, err := json.Marshal(&resp)
	if err != nil {
		return err
	}
	pushRegistorJob(data["inbox_url"], jsonData)
	relayState.RedisClient.Del("relay:autopending:" + info.Domain)
	return nil

}

func unfollowDomain(info conf.SubscriptionInfo) error {

	data, err := relayState.RedisClient.HGetAll("relay:subscription:" + info.Domain).Result()
	if err != nil {
		return err
	}
	activity := activitypub.Activity{
		Context: []string{"https://www.w3.org/ns/activitystreams", "https://w3id.org/security/v1"},
		ID:      data["activity_id"],
		Actor:   data["actor_id"],
		Type:    "Follow",
		Object:  "https://www.w3.org/ns/activitystreams#Public",
	}
	resp := activity.GenerateResponse(hostURL, "Reject")
	jsonData, _ := json.Marshal(&resp)
	pushRegistorJob(data["inbox_url"], jsonData)
	if info.Software != "mastodon" {
		fb := activity.GeneratebackRequest(hostURL, "Undo")
		fbjsonData, _ := json.Marshal(&fb)
		go pushRegistorJob(data["inbox_url"], fbjsonData)
		starlog.Infoln("Send Follow Back Request : ", activity.Actor)
	}
	relayState.DelSubscription(info.Domain)
	return nil
}
