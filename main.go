package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/kwilteam/cometseed/crawler"

	"github.com/BurntSushi/toml"
	"github.com/cometbft/cometbft/libs/log"
)

var (
	logger = log.NewTMLogger(log.NewSyncWriter(os.Stdout))
)

const (
	configDir = ".cometseed"
)

type conf struct {
	ChainID string `toml:"chain_id" comment:"network identifier"`
	Seeds   string `toml:"seeds" comment:"seed nodes we can use to discover peers"`

	ListenAddress       string `toml:"listen" comment:"address to listen for incoming connections"`
	NodeKeyFile         string `toml:"node_key_file" comment:"path to node_key (relative to app root directory or an absolute path)"`
	AddrBookFile        string `toml:"addr_book_file" comment:"path to address book (relative to app root directory or an absolute path)"`
	AddrBookStrict      bool   `toml:"addr_book_strict" comment:"use strict routability rules (keep false for private or local networks)"`
	MaxNumInboundPeers  int    `toml:"max_inbound" comment:"maximum number of inbound connections"`
	MaxNumOutboundPeers int    `toml:"max_outbound" comment:"maximum number of outbound connections"`
}

func defaultConfig() *conf {
	return &conf{
		ChainID: "",
		Seeds:   "",

		ListenAddress:       "tcp://0.0.0.0:26656",
		NodeKeyFile:         "node_key.json",
		AddrBookFile:        "addrbook.json",
		AddrBookStrict:      false,
		MaxNumInboundPeers:  1000,
		MaxNumOutboundPeers: 300,
	}
}

func main() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	rootDir := filepath.Join(homeDir, configDir)
	err = os.MkdirAll(rootDir, 0700)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	cfg := defaultConfig()
	configFile := filepath.Join(rootDir, "config.toml")
	if fid, err := os.Open(configFile); err == nil {
		_, err = toml.NewDecoder(fid).Decode(&cfg) // _, err = toml.DecodeFile(configFile, &cfg)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fid.Close()
	} else if !os.IsNotExist(err) {
		fmt.Println(err)
		os.Exit(1)
	} // else defaults

	if idEnv := os.Getenv("CHAIN"); idEnv != "" {
		cfg.ChainID = idEnv
	}
	if seedsEnv := os.Getenv("SEEDS"); seedsEnv != "" {
		cfg.Seeds = seedsEnv
	}

	flag.StringVar(&cfg.ChainID, "chain-id", cfg.ChainID, "chain ID")
	flag.StringVar(&cfg.Seeds, "seeds", cfg.Seeds, "seed nodes")
	flag.StringVar(&cfg.ListenAddress, "listen-addr", cfg.ListenAddress, "P2P listen address")
	flag.Parse()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		<-signalChan
		cancel()
	}()

	crl, err := crawler.NewCrawler(ctx, rootDir, logger, (*crawler.Conf)(cfg))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if err = crl.Crawl(ctx); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	os.Exit(0)
}
