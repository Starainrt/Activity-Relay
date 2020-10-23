package conf

import (
	"io/ioutil"
	"strings"

	"b612.me/starlog"
	"b612.me/starmap"
	"b612.me/staros/sysconf"
)

type RelayConfig struct {
	Domain       string `seg:"relay" key:"domain"`
	Listen       string `seg:"relay" key:"listen"`
	Redis        string `seg:"relay" key:"redis"`
	Key          string `seg:"relay" key:"private_key"`
	Name         string `seg:"relay" key:"name"`
	Summary      string `seg:"relay" key:"summary"`
	Icon         string `seg:"relay" key:"icon"`
	Image        string `seg:"relay" key:"image"`
	ManualAccept bool   `seg:"relay" key:"manual_accept"`

	SupportAuthorized bool   `seg:"rule" key:"support_authorized_fetch"`
	MaxInstances      int    `seg:"rule" key:"allow_max_instances"`
	AllowRule         string `seg:"rule" key:"allow_rule_file"`
	BlockRule         string `seg:"rule" key:"block_rule_file"`
	UserByActive      bool   `seg:"rule" key:"user_by_active"`
	AllowMaxUser      int    `seg:"rule" key:"allow_max_user"`
	AllowMinUser      int    `seg:"rule" key:"allow_min_user"`
	KickMaxUser       int    `seg:"rule" key:"kick_max_user"`
	KickMinUser       int    `seg:"rule" key:"kick_min_user"`
	AllowGeo          string `seg:"rule" key:"allow_geo"`
	BlockGeo          string `seg:"rule" key:"block_geo"`
	IPSource          string `seg:"rule" key:"ip_source"`
	UpdateRate        int    `seg:"rule" key:"update_rate"`
	KickAfter         int    `seg:"rule" key:"kick_after_no_response_times"`

	LogLevel int `seg:"log" key:"level"`

	Version string
}

func Parse(path string) error {
	var cfg RelayConfig
	ini := sysconf.NewSysConf("=")
	ini.HaveSegMent = true
	ini.SegStart = "["
	ini.SegEnd = "]"
	ini.SpaceStr = " "
	ini.CommentCR = true
	ini.CommentFlag = []string{";", "#"}
	ini.EscapeFlag = "\\"
	err := ini.ParseFromFile(path)
	if err != nil {
		return err
	}
	err = ini.Unmarshal(&cfg)
	if err != nil {
		return err
	}
	err = starmap.Store("config", cfg)
	starlog.Std.LogLevel = cfg.LogLevel
	if cfg.AllowRule != "" {
		allowList, err := parseLists(cfg.AllowRule)
		if err != nil {
			starlog.Errorln("Cannot Parse Allow List:", err)
		} else {
			starmap.Store("allowlist", allowList)
		}
	} else {
		starmap.Store("allowlist", []string{})
	}

	if cfg.BlockRule != "" {
		blockList, err := parseLists(cfg.BlockRule)
		if err != nil {
			starlog.Errorln("Cannot Parse Block List:", err)
		} else {
			starmap.Store("blocklist", blockList)
		}
	} else {
		starmap.Store("blocklist", []string{})
	}
	return err
}

func parseLists(path string) ([]string, error) {
	var result []string
	allowBit, err := ioutil.ReadFile(path)
	if err != nil {
		return result, err
	}
	list := strings.Split(string(allowBit), "\n")
	for _, v := range list {
		data := strings.TrimSpace(v)
		if data != "" {
			result = append(result, data)
		}
	}
	return result, nil
}
