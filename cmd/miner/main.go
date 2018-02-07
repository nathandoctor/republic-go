package main

import (
	"errors"
	"flag"
	"log"
	"net"
	"time"

	"github.com/republicprotocol/go-miner"
)

var config *miner.Config

func main() {
	if err := parseCommandLineFlags(); err != nil {
		log.Println(err)
		flag.Usage()
		return
	}

	miner, err := miner.NewMiner(config)
	if err != nil {
		log.Fatal(err)
	}
	listener, err := net.Listen("tcp", config.Host+":"+config.Port)
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		if err := miner.Xing.Serve(listener); err != nil {
			log.Fatal(err)
		}
	}()
	go func() {
		if err := miner.Swarm.Serve(listener); err != nil {
			log.Fatal(err)
		}
	}()
	time.Sleep(time.Second)

	go func() {
		log.Println("establishing connections...")
		miner.EstablishConnections()
	}()

	log.Println("digging...")
	quit := make(chan struct{})
	miner.Mine(quit)
}

func parseCommandLineFlags() error {
	confFilename := flag.String("config", "", "Path to the JSON configuration file")

	flag.Parse()

	if *confFilename == "" {
		return errors.New("no config file given")
	}

	conf, err := miner.LoadConfig(*confFilename)
	if err != nil {
		return err
	}
	config = conf

	return nil
}
