package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/cometbft/cometbft/config"
	"github.com/cometbft/cometbft/libs/log"
	tmstrings "github.com/cometbft/cometbft/libs/strings"
	"github.com/cometbft/cometbft/p2p"
	"github.com/cometbft/cometbft/p2p/pex"
	"github.com/cometbft/cometbft/version"
)

var (
	logger = log.NewTMLogger(log.NewSyncWriter(os.Stdout))
)

const (
	ver       = "0.6.1"
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
		MaxNumInboundPeers:  3000,
		MaxNumOutboundPeers: 1000,
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
	flag.Parse()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		<-signalChan
		cancel()
	}()

	if err = crawl(ctx, rootDir, cfg); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	os.Exit(0)
}

func crawl(ctx context.Context, rootDir string, cfg *conf) error {
	nodeKeyFilePath := filepath.Join(rootDir, cfg.NodeKeyFile)
	nodeKey, err := p2p.LoadOrGenNodeKey(nodeKeyFilePath)
	if err != nil {
		return err
	}

	chainID, nodeID := cfg.ChainID, nodeKey.ID()
	logger.Info("config", "node ID", nodeID, "chain", chainID, "listen addr", cfg.ListenAddress)

	nodeInfo := p2p.DefaultNodeInfo{
		ProtocolVersion: p2p.NewProtocolVersion(version.P2PProtocol, version.BlockProtocol, 0),
		DefaultNodeID:   nodeID,
		ListenAddr:      cfg.ListenAddress,
		Network:         chainID,
		Version:         ver,
		Channels:        []byte{pex.PexChannel},
		Moniker:         chainID + "-seeder",
	}

	addr, err := p2p.NewNetAddressString(p2p.IDAddressString(nodeInfo.DefaultNodeID, nodeInfo.ListenAddr))
	if err != nil {
		return err
	}

	p2pConf := config.DefaultP2PConfig()
	p2pConf.AllowDuplicateIP = true

	// Listen for incoming p2p
	transport := p2p.NewMultiplexTransport(nodeInfo, *nodeKey, p2p.MConnConfig(p2pConf))
	if err := transport.Listen(*addr); err != nil {
		return err
	}
	defer transport.Close()

	// Load address book
	filteredLogger := log.NewFilter(logger, log.AllowInfo())
	addrBookFilePath := filepath.Join(rootDir, cfg.AddrBookFile)
	book := pex.NewAddrBook(addrBookFilePath, cfg.AddrBookStrict)
	book.SetLogger(filteredLogger.With("module", "book"))

	// Create PEX reactor
	pexReactor := pex.NewReactor(book, &pex.ReactorConfig{
		SeedMode:                 true, // just crawl and hang up on incoming after pex
		Seeds:                    tmstrings.SplitAndTrim(cfg.Seeds, ",", " "),
		SeedDisconnectWaitPeriod: 5 * time.Minute,
	})
	pexReactor.SetLogger(filteredLogger.With("module", "pex"))

	// Start p2p switch
	sw := p2p.NewSwitch(p2pConf, transport)
	sw.SetLogger(filteredLogger.With("module", "switch"))
	sw.SetNodeKey(nodeKey)
	sw.SetAddrBook(book)
	sw.AddReactor("pex", pexReactor)
	sw.SetNodeInfo(nodeInfo)

	if err = sw.Start(); err != nil {
		return err
	}

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

loop:
	for {
		select {
		case <-ticker.C:
			logger.Debug("", "peers", sw.Peers().List())
		case <-ctx.Done():
			logger.Info("Saving address book")
			book.Save()
			logger.Info("Stopping p2p switch")
			if err := sw.Stop(); err != nil {
				return err
			}
			break loop
		}
	}

	return nil
}
