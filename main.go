package main

import (
	"context"
	"errors"
	"io/ioutil"
	"os"
	"os/signal"
	"syscall"

	"fmt"

	"b612.me/starmap"

	"github.com/starainrt/Activity-Relay/cli"
	"github.com/starainrt/Activity-Relay/conf"
	"github.com/starainrt/Activity-Relay/server"
	"github.com/starainrt/Activity-Relay/worker"

	"b612.me/starlog"
	"b612.me/staros"
	"github.com/spf13/cobra"
)

func init() {
	cli.BuildNewCmd(cmdStart)
	cmdStart.Flags().StringP("config", "c", "./config/config.ini", "Configure Path")
}

func main() {
	cmdStart.Execute()
}

var cmdStart = &cobra.Command{
	Use:   "",
	Short: "Just a Small Relay",
	Long:  "Relay",
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("config")
		err := loadConfigure(configPath)
		if err != nil {
			starlog.Errorln(err)
			return err
		}
		stopCtx, stopFn := context.WithCancel(context.Background())
		workChan := make(chan int)

		err = ioutil.WriteFile("./config/relay.pid", []byte(fmt.Sprint(os.Getpid())), 0755)
		if err != nil {
			starlog.Errorln("Cannot Write Pid File", err)
			return err
		}
		go server.Run(stopCtx)
		go worker.Run(workChan)
		sig := make(chan os.Signal)
		signal.Notify(sig, os.Interrupt, os.Kill)
		reloadSig := make(chan os.Signal)
		signal.Notify(reloadSig, syscall.SIGUSR1)
		for {
			select {
			case <-sig:
				stopFn()
				<-workChan
				return nil
			case <-workChan:
				panic("quit unexpect!")
			case <-reloadSig:
				starlog.Infoln("Ok,Recv Reload Cfg Sig,Please Wait")
				starlog.Noticeln("Please Note: Only Rules Can be Updated!")
				err := loadConfigure(configPath)
				if err != nil {
					starlog.Errorln(err)
				}
				starlog.Infoln("Load Config Success!")
			}
		}

		return nil
	},
}

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
