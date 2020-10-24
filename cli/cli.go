package cli

import (
	"crypto/rsa"
	"errors"
	"fmt"
	"net/url"
	"os"

	"b612.me/starlog"
	"b612.me/starmap"
	"b612.me/staros"
	"github.com/RichardKnop/machinery/v1"
	"github.com/RichardKnop/machinery/v1/config"
	"github.com/go-redis/redis"
	"github.com/spf13/cobra"
	activitypub "github.com/starainrt/Activity-Relay/ActivityPub"
	keyloader "github.com/starainrt/Activity-Relay/KeyLoader"
	"github.com/starainrt/Activity-Relay/conf"
)

var (
	version string

	// Actor : Relay's Actor
	Actor activitypub.Actor

	hostname        *url.URL
	hostkey         *rsa.PrivateKey
	relayState      conf.RelayState
	machineryServer *machinery.Server
)

func loadConfigure(configPath string) error {
	if !staros.Exists(configPath) {
		starlog.Criticalln("Cannot Found Config File,Please Check")
		return errors.New("Config File Not Exists")
	}
	starlog.Noticeln("Parsing Config File……")
	if err := conf.Parse(configPath); err != nil {
		starlog.Criticalln("Cannot Parse Configure File:", err)
		return err
	}
	cfg := starmap.MustGet("config").(conf.RelayConfig)
	cfg.Version = "1.0.0"
	starmap.Store("ua", fmt.Sprintf("ActivityPub-Relay V%s +https://%s", cfg.Version, cfg.Domain))
	starmap.Store("config", cfg)
	fmt.Printf("Load Config :%+v\n", cfg)
	return nil
}

func initConfig() {
	err := loadConfigure("./config/config.ini")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	cfg := starmap.MustGet("config").(conf.RelayConfig)
	Actor.Summary = cfg.Summary
	Actor.Icon = activitypub.Image{URL: cfg.Icon}
	Actor.Image = activitypub.Image{URL: cfg.Image}
	Actor.Name = cfg.Name

	hostname, err = url.Parse("https://" + cfg.Domain)
	if err != nil {
		panic(err)
	}
	hostkey, err := keyloader.ReadPrivateKeyRSAfromPath(cfg.Key)
	if err != nil {
		panic(err)
	}
	redisOption, err := redis.ParseURL(cfg.Redis)
	if err != nil {
		panic(err)
	}
	redisClient := redis.NewClient(redisOption)
	relayState = conf.NewState(redisClient, false)
	var machineryConfig = &config.Config{
		Broker:          cfg.Redis,
		DefaultQueue:    "relay",
		ResultBackend:   cfg.Redis,
		ResultsExpireIn: 5,
	}
	machineryServer, err = machinery.NewServer(machineryConfig)
	if err != nil {
		panic(err)
	}

	Actor.GenerateSelfKey(hostname, &hostkey.PublicKey)
}

func BuildNewCmd(app *cobra.Command) {

	//app.PersistentFlags().StringP("config", "c", "./config.ini", "Config Path")
	app.AddCommand(domainCmdInit())
	app.AddCommand(followCmdInit())
	app.AddCommand(configCmdInit())
}
