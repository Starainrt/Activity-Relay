package server

import (
	"net/http"
	"text/template"
	"time"

	"b612.me/starmap"
	"github.com/starainrt/Activity-Relay/conf"
)

type Status struct {
	Info []conf.SubscriptionInfo
	Date string
	Num  int
	Name string
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	cfg := starmap.MustGet("config").(conf.RelayConfig)
	relayInfo := GetDomainList()
	t, _ := template.ParseFiles("./config/index.html")
	var status Status
	status.Num = len(relayInfo)
	status.Info = relayInfo
	status.Date = time.Now().Format("2006-01-02 15:04:05 +0000 MST")
	status.Name = cfg.Name
	t.Execute(w, status)
}

func GetDomainList() []conf.SubscriptionInfo {
	var lists []conf.SubscriptionInfo
	for _, domain := range relayState.Subscriptions {
		info := GetInstancesInfo(domain.Domain)
		info.Domain = domain.Domain
		lists = append(lists, info)
	}
	return lists
}
