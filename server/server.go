package server

import (
	"context"
	"crypto/rsa"
	"net/http"
	"net/url"
	"time"

	"b612.me/starlog"
	"b612.me/starmap"
	"github.com/RichardKnop/machinery/v1"
	"github.com/RichardKnop/machinery/v1/config"
	"github.com/go-redis/redis"
	cache "github.com/patrickmn/go-cache"
	activitypub "github.com/starainrt/Activity-Relay/ActivityPub"
	keyloader "github.com/starainrt/Activity-Relay/KeyLoader"
	"github.com/starainrt/Activity-Relay/conf"
)

var (
	version string

	// Actor : Relay's Actor
	Actor activitypub.Actor

	// WebfingerResource : Relay's Webfinger resource
	WebfingerResource activitypub.WebfingerResource

	// Nodeinfo : Relay's Nodeinfo
	Nodeinfo activitypub.NodeinfoResources

	hostURL         *url.URL
	hostPrivatekey  *rsa.PrivateKey
	relayState      conf.RelayState
	machineryServer *machinery.Server
	actorCache      *cache.Cache
)

func serverInit() error {
	cfg := starmap.MustGet("config").(conf.RelayConfig)
	Actor.Summary = cfg.Summary
	Actor.Icon = activitypub.Image{URL: cfg.Icon}
	Actor.Image = activitypub.Image{URL: cfg.Image}
	Actor.Name = cfg.Name
	hostURL, _ = url.Parse("https://" + cfg.Domain)
	hostPrivatekey, _ = keyloader.ReadPrivateKeyRSAfromPath(cfg.Key)
	redisOption, err := redis.ParseURL(cfg.Redis)
	if err != nil {
		panic(err)
	}
	redisClient := redis.NewClient(redisOption)
	relayState = conf.NewState(redisClient, true)
	relayState.ListenNotify(nil)
	machineryConfig := &config.Config{
		Broker:          cfg.Redis,
		DefaultQueue:    "relay",
		ResultBackend:   cfg.Redis,
		ResultsExpireIn: 5,
	}
	machineryServer, err = machinery.NewServer(machineryConfig)
	if err != nil {
		panic(err)
	}

	Actor.GenerateSelfKey(hostURL, &hostPrivatekey.PublicKey)
	actorCache = cache.New(5*time.Minute, 10*time.Minute)
	WebfingerResource.GenerateFromActor(hostURL, &Actor)
	Nodeinfo.GenerateFromActor(hostURL, &Actor, version)

	return nil
}

func Run(stopCtx context.Context) {
	serverInit()
	cfg := starmap.MustGet("config").(conf.RelayConfig)
	if cfg.ManualAccept {
		relayState.SetConfig(1, true)
		starlog.Infoln("ManuallyAccept Enabled")
	} else {
		relayState.SetConfig(1, false)
		starlog.Infoln("ManuallyAccept Disabled")
	}
	httpServer := &http.Server{Addr: cfg.Listen}
	http.HandleFunc("/.well-known/nodeinfo", handleNodeinfoLink)
	http.HandleFunc("/.well-known/webfinger", handleWebfinger)
	http.HandleFunc("/nodeinfo/2.1", handleNodeinfo)
	http.HandleFunc("/actor", handleActor)
	http.HandleFunc("/inbox", func(w http.ResponseWriter, r *http.Request) {
		handleInbox(w, r, decodeActivity)
	})
	http.HandleFunc("/", handleIndex)
	go UpdateInstancesProcess(stopCtx)
	//go updateAllInstancesInfo()
	go RecheckInstanceAcceptProcess(stopCtx)
	go KickInstancesProcess(stopCtx)
	go func() {
		starlog.Infof("Now Listening On:%s\n", cfg.Listen)
		if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
			starlog.Errorln("unexpect error: %v", err)
		}
	}()
	<-stopCtx.Done()
	if err := httpServer.Shutdown(context.TODO()); err != nil {
		starlog.Errorln("unexpect error: %v", err)
		panic(err)
	}
	starlog.Infoln("Shutdown Ok")
}
