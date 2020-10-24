package conf

import (
	"encoding/json"
	"errors"
	"fmt"

	"b612.me/starmap"
	"b612.me/starnet"
)

type MetaData struct {
	OpenReg  bool      `json:"openRegistrations"`
	Software SoftWares `json:"software"`
	Usage    Usages    `json:"usage"`
}

type SoftWares struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type Usages struct {
	Posts int   `json:"localPosts"`
	User  Users `json:"users"`
}

type Users struct {
	Total  int `json:"total"`
	Active int `json:"activeMonth"`
}

func UpdateInstancesInfo(domain string) (SubscriptionInfo, error) {
	defer func() {
		recover()
	}()
	var wellKnown map[string]interface{}
	var metadata MetaData
	var info SubscriptionInfo
	wellknownUrl := "https://" + domain + "/.well-known/nodeinfo"
	req := starnet.NewRequests(wellknownUrl, nil, "GET")
	req.ReqHeader.Set("User-Agent", starmap.MustGet("ua").(string))
	req.ReqHeader.Set("Accept", "application/json")
	data, err := starnet.Curl(req)
	if err != nil {
		return info, err
	}
	if data.RespHttpCode/100 != 2 {
		return info, fmt.Errorf("Server Returned Invaild Code %d While Get Instances Info %s", data.RespHttpCode, wellknownUrl)
	}
	err = json.Unmarshal(data.RecvData, &wellKnown)
	refData, ok := wellKnown["links"]
	if !ok {
		return info, errors.New("Cannot Get Meta Data From Remote Instances")
	}
	hrefData, ok := refData.([]interface{})[0].(map[string]interface{})["href"]
	if !ok {
		return info, errors.New("Cannot Get Href Data From Remote Instances")
	}

	req = starnet.NewRequests(hrefData.(string), nil, "GET")
	req.ReqHeader.Set("User-Agent", starmap.MustGet("ua").(string))
	req.ReqHeader.Set("Accept", "application/activity+json")
	data, err = starnet.Curl(req)
	if err != nil {
		return info, err
	}
	if data.RespHttpCode/100 != 2 {
		return info, fmt.Errorf("Server Returned Invaild Code %d While Get Instances Info %s", data.RespHttpCode, hrefData.(string))
	}
	err = json.Unmarshal(data.RecvData, &metadata)
	if err != nil {
		return info, err
	}
	info.Domain = domain
	info.Software = metadata.Software.Name
	info.Version = metadata.Software.Version
	info.TotalUser = metadata.Usage.User.Total
	info.ActiveUser = metadata.Usage.User.Active
	if info.ActiveUser == 0 {
		info.ActiveUser = info.TotalUser
	}
	info.Post = metadata.Usage.Posts
	return info, nil
}
