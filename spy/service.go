package main

import (
	"context"
	"fmt"
	"regexp"
	"time"
)

func DomainPermit(stopCtx context.Context) {
	for {
		select {
		case <-stopCtx.Done():
			return
		case <-time.After(time.Second * 5):
		}
		followReq, err := listFollows()
		if err != nil {
			log("Cannot Get Follow List", err)
			continue
		}

		domains, _ := GetDomainList()
		if len(domains) > conf.maxInstances {
			log("Cannot Permit New Instance,Existing Instances is Too Much!")
			for _, domain := range followReq {
				err := rejectFollow(domain)
				if err != nil {
					log("Cannot reject "+domain, err)
					continue
				}
				log(fmt.Sprintf("Instance %s Rejected", domain))
			}
		}

		log("Got " + fmt.Sprint(len(followReq)) + " New Relay Follow Requests")
		for _, domain := range followReq {
			if !shouldPermit(domain) {
				log(fmt.Sprintf("Cannot Permit %s Due to blacklist/whitelist policy", domain))
				err := rejectFollow(domain)
				if err != nil {
					log("Cannot reject "+domain, err)
					continue
				}
				log(fmt.Sprintf("Instance %s Rejected", domain))
				continue
			}
			num := CheckInstanceNum(domain, conf.byTotal)
			if num < 0 {
				log(fmt.Sprintf("Cannot Get Instance %s Num", domain))
				continue
			}
			if conf.allowMaxUser == 0 || (num > conf.allowMinUser && num < conf.allowMaxUser) {
				err := acceptFollow(domain)
				if err != nil {
					log("Cannot Permit "+domain, err)
					continue
				}
				log(fmt.Sprintf("Instance %s Permited,Count %d", domain, num))
			} else {
				err := rejectFollow(domain)
				if err != nil {
					log("Cannot reject "+domain, err)
					continue
				}
				log(fmt.Sprintf("Instance %s Rejected,Count %d", domain, num))
			}
		}
	}
}

func DomainReview(stopCtx context.Context) {
	for {
		select {
		case <-stopCtx.Done():
			return
		case <-time.After(time.Second * 60):
		}

		domains, err := GetDomainList()
		if err != nil {
			log("Cannot Get Domain Lists", err)
			continue
		}

		for _, domain := range domains {
			if !shouldPermit(domain) {
				log(fmt.Sprintf("Domain %s Should Kick Due to Blacklist/whitelist Policy", domain))
				err := unfollowDomains(domain)
				if err != nil {
					log("Cannot kick "+domain, err)
					continue
				}
				log(fmt.Sprintf("Kick Domain %s Succeed", domain))
			}
			num := CheckInstanceNum(domain, conf.byTotal)
			if num < 0 {
				log(fmt.Sprintf("Cannot Get Instance %s Num", domain))
				continue
			}
			if (conf.kickMaxUser != 0 && num > conf.kickMaxUser) || (conf.kickMinUser != 0 && num < conf.kickMinUser) {
				log(fmt.Sprintf("Domain %s Should Kick,count is %d", domain, num))
				err := unfollowDomains(domain)
				if err != nil {
					log("Cannot Kick "+domain, err)
					continue
				}
				log(fmt.Sprintf("Kick Domain %s Succeed", domain))
			}
		}
	}
}

func shouldPermit(domain string) bool {
	if !conf.whitelistMode && !conf.blacklistMode {
		return true
	}
	if conf.whitelistMode {
		for _, rep := range conf.whitelist {
			should, err := regexp.MatchString(rep, domain)
			if err != nil {
				continue
			}
			if should {
				return true
			}
		}
		return false
	}

	if conf.blacklistMode {
		for _, rep := range conf.blacklist {
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
	return false
}
