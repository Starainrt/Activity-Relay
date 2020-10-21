package main

import (
	"context"
	"crypto/rsa"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/RichardKnop/machinery/v1"
	"github.com/RichardKnop/machinery/v1/config"
	"github.com/go-redis/redis"
	"github.com/spf13/viper"
	activitypub "github.com/yukimochi/Activity-Relay/ActivityPub"
	keyloader "github.com/yukimochi/Activity-Relay/KeyLoader"
	state "github.com/yukimochi/Activity-Relay/State"
)

const (
	BlockService state.Config = iota
	ManuallyAccept
	CreateAsAnnounce
)

type configs struct {
	allowMaxUser  int
	allowMinUser  int
	kickMaxUser   int
	kickMinUser   int
	maxInstances  int
	permitMode    bool
	whitelistMode bool
	blacklistMode bool
	whitelist     []string
	blacklist     []string
	byTotal       bool
}

var (
	version string
	conf    configs
	// Actor : Relay's Actor
	Actor activitypub.Actor

	hostname        *url.URL
	hostkey         *rsa.PrivateKey
	relayState      state.RelayState
	machineryServer *machinery.Server
)
var redisClient *redis.Client

func main() {
	time.Sleep(time.Second * 2)
	initConfig()
	sig := make(chan os.Signal)
	signal.Notify(sig, os.Interrupt, os.Kill)
	stopCtx, stopFn := context.WithCancel(context.Background())
	go DomainPermit(stopCtx)
	go DomainReview(stopCtx)
	<-sig
	log("Recv Signal,Now Exited")
	stopFn()
	redisClient.Close()
}

func initConfig() {
	viper.SetConfigName("config")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()
	if err != nil {
		log(err)
		fmt.Println("Config file is not exists or parse error. Use environment variables.")
		viper.BindEnv("allow_max_user")
		viper.BindEnv("allow_min_user")
		viper.BindEnv("kick_max_user")
		viper.BindEnv("kick_min_user")
		viper.BindEnv("by_total")
	} else {
		Actor.Summary = viper.GetString("relay_summary")
		Actor.Icon = activitypub.Image{URL: viper.GetString("relay_icon")}
		Actor.Image = activitypub.Image{URL: viper.GetString("relay_image")}
	}
	conf.allowMaxUser = viper.GetInt("allow_max_user")
	conf.allowMinUser = viper.GetInt("allow_min_user")
	conf.kickMaxUser = viper.GetInt("kick_max_user")
	conf.kickMinUser = viper.GetInt("kick_min_user")
	conf.maxInstances = viper.GetInt("max_instances")
	conf.byTotal = viper.GetBool("user_by_total")
	conf.permitMode = viper.GetBool("permit_mode")
	conf.whitelistMode = viper.GetBool("whitelist_mode")
	conf.blacklistMode = viper.GetBool("blacklist_mode")
	conf.whitelist = viper.GetStringSlice("whitelist")
	conf.blacklist = viper.GetStringSlice("blacklist")

	Actor.Name = viper.GetString("relay_servicename")
	log(fmt.Printf("%+v", conf))
	hostname, err = url.Parse("https://" + viper.GetString("relay_domain"))
	if err != nil {
		panic(err)
	}
	hostkey, err := keyloader.ReadPrivateKeyRSAfromPath(viper.GetString("actor_pem"))
	if err != nil {
		panic(err)
	}
	redisOption, err := redis.ParseURL(viper.GetString("redis_url"))
	if err != nil {
		panic(err)
	}
	redisClient = redis.NewClient(redisOption)
	relayState = state.NewState(redisClient, false)
	if !conf.permitMode {
		relayState.SetConfig(ManuallyAccept, false)
		log("Manually accept follow-request is Disabled.")
	} else {
		relayState.SetConfig(ManuallyAccept, true)
		log("Manually accept follow-request is Enabled.")
	}

	var machineryConfig = &config.Config{
		Broker:          viper.GetString("redis_url"),
		DefaultQueue:    "relay",
		ResultBackend:   viper.GetString("redis_url"),
		ResultsExpireIn: 5,
	}
	machineryServer, err = machinery.NewServer(machineryConfig)
	if err != nil {
		panic(err)
	}

	Actor.GenerateSelfKey(hostname, &hostkey.PublicKey)
	log("Service Start……")
}

func log(data ...interface{}) {
	dateStr := time.Now().Format("2006-01-02 15:04:05 ")
	var print []interface{}
	print = append(print, dateStr)
	for _, v := range data {
		print = append(print, v)
	}
	fmt.Println(print...)
}
