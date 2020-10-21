package main

import (
	"net/http"
	"strings"
	"text/template"
	"time"
)

var relayInfo Info

type Info struct {
	Name   string
	Desp   string
	Domain []string
	Num    int
	Update string
}

func updateWebInfo() {
	for {
		domains, err := GetDomainList()
		if err != nil {
			time.Sleep(time.Second * 20)
			continue
		}
		relayInfo.Domain = domains
		relayInfo.Name = Actor.Name
		relayInfo.Desp = Actor.Summary
		relayInfo.Num = len(domains)
		relayInfo.Update = time.Now().Format("2006-01-02 15:04:05 -0700 MST")
		time.Sleep(time.Minute * 10)
	}
}

func HandleIndex(w http.ResponseWriter, r *http.Request) {
	t, _ := template.ParseFiles("./index.html")
	t.Execute(w, relayInfo)
}

func GetDomainList() ([]string, error) {
	var lists []string
	domains, err := relayState.RedisClient.Keys("relay:subscription:*").Result()
	if err != nil {
		return lists, err
	}
	for _, domain := range domains {
		domainName := strings.Replace(domain, "relay:subscription:", "", 1)
		lists = append(lists, domainName)
	}
	return lists, nil
}
