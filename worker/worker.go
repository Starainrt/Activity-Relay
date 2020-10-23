package worker

import (
	"crypto/rsa"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"b612.me/starlog"
	"b612.me/starmap"
	"github.com/RichardKnop/machinery/v1"
	"github.com/RichardKnop/machinery/v1/config"
	"github.com/RichardKnop/machinery/v1/log"
	"github.com/go-redis/redis"
	uuid "github.com/satori/go.uuid"
	activitypub "github.com/starainrt/Activity-Relay/ActivityPub"
	keyloader "github.com/starainrt/Activity-Relay/KeyLoader"
	"github.com/starainrt/Activity-Relay/conf"
)

var (
	version string

	// Actor : Relay's Actor
	Actor activitypub.Actor

	hostURL         *url.URL
	hostPrivatekey  *rsa.PrivateKey
	redisClient     *redis.Client
	machineryServer *machinery.Server
	httpClient      *http.Client
)

func relayActivity(args ...string) error {
	inboxURL := args[0]
	body := args[1]
	err := sendActivity(inboxURL, Actor.ID, []byte(body), hostPrivatekey)
	if err != nil {
		domain, _ := url.Parse(inboxURL)
		mod, _ := redisClient.HSetNX("relay:statistics:"+domain.Host, "last_error", err.Error()).Result()
		if mod {
			redisClient.Expire("relay:statistics:"+domain.Host, time.Duration(time.Minute))
		}
	}
	return err
}

func registorActivity(args ...string) error {
	inboxURL := args[0]
	body := args[1]
	err := sendActivity(inboxURL, Actor.ID, []byte(body), hostPrivatekey)
	return err
}

func initConfig() {
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

	fmt.Printf("%v\n", cfg)
	fmt.Println(cfg.Redis)

	redisClient = redis.NewClient(redisOption)
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
	httpClient = &http.Client{Timeout: time.Duration(5) * time.Second}

	Actor.GenerateSelfKey(hostURL, &hostPrivatekey.PublicKey)
	newNullLogger := starlog.New(os.Stdout)
	log.DEBUG = newNullLogger

}

func Run(workChan chan int) {
	initConfig()

	err := machineryServer.RegisterTask("registor", registorActivity)
	if err != nil {
		panic(err.Error())
	}
	err = machineryServer.RegisterTask("relay", relayActivity)
	if err != nil {
		panic(err.Error())
	}

	workerID := uuid.NewV4()
	worker := machineryServer.NewWorker(workerID.String(), 200)
	err = worker.Launch()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	workChan <- 1
}
