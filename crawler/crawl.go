package crawler

import (
	"context"
	"path/filepath"
	"time"

	"github.com/cometbft/cometbft/config"
	"github.com/cometbft/cometbft/libs/log"
	cmtstrings "github.com/cometbft/cometbft/libs/strings"
	"github.com/cometbft/cometbft/p2p"
	"github.com/cometbft/cometbft/p2p/pex"
	"github.com/cometbft/cometbft/version"
)

const ver = "0.8.0"

type Conf struct {
	ChainID             string
	Seeds               string
	ListenAddress       string
	NodeKeyFile         string
	AddrBookFile        string
	AddrBookStrict      bool
	MaxNumInboundPeers  int
	MaxNumOutboundPeers int
}

type Crawler struct {
	addrBook  pex.AddrBook
	nodeInfo  p2p.DefaultNodeInfo
	nodeKey   *p2p.NodeKey
	transport *p2p.MultiplexTransport
	sw        *p2p.Switch
	logger    log.Logger
}

func NewCrawler(ctx context.Context, rootDir string, logger log.Logger, cfg *Conf) (*Crawler, error) {
	logger.Info("Starting crawler", "root dir", rootDir, "config", cfg)
	// Node key
	nodeKeyFilePath := filepath.Join(rootDir, cfg.NodeKeyFile)
	nodeKey, err := p2p.LoadOrGenNodeKey(nodeKeyFilePath)
	if err != nil {
		return nil, err
	}

	chainID, nodeID := cfg.ChainID, nodeKey.ID()
	logger.Info("config", "node ID", nodeID, "chain", chainID, "listen addr", cfg.ListenAddress)

	// Node Info
	nodeInfo := p2p.DefaultNodeInfo{
		ProtocolVersion: p2p.NewProtocolVersion(version.P2PProtocol, version.BlockProtocol, 0),
		DefaultNodeID:   nodeID,
		ListenAddr:      cfg.ListenAddress,
		Network:         chainID,
		Version:         ver,
		Channels:        []byte{pex.PexChannel},
		Moniker:         chainID + "-seeder",
	}

	p2pConf := config.DefaultP2PConfig()
	p2pConf.AllowDuplicateIP = true
	p2pConf.MaxNumInboundPeers = cfg.MaxNumInboundPeers
	p2pConf.MaxNumOutboundPeers = cfg.MaxNumOutboundPeers

	transport := p2p.NewMultiplexTransport(nodeInfo, *nodeKey, p2p.MConnConfig(p2pConf))

	filteredLogger := log.NewFilter(logger, log.AllowInfo())
	addrBookFilePath := filepath.Join(rootDir, cfg.AddrBookFile)
	book := pex.NewAddrBook(addrBookFilePath, cfg.AddrBookStrict)
	book.SetLogger(filteredLogger.With("module", "book"))

	// Create PEX reactor
	pexReactor := pex.NewReactor(book, &pex.ReactorConfig{
		SeedMode:                 true, // just crawl and hang up on incoming after pex
		Seeds:                    cmtstrings.SplitAndTrim(cfg.Seeds, ",", " "),
		SeedDisconnectWaitPeriod: 3 * time.Minute,
	})
	pexReactor.SetLogger(filteredLogger.With("module", "pex"))

	// Create p2p switch
	sw := p2p.NewSwitch(p2pConf, transport)
	sw.SetLogger(filteredLogger.With("module", "switch"))
	sw.SetNodeKey(nodeKey)
	sw.SetAddrBook(book)
	sw.AddReactor("pex", pexReactor)
	sw.SetNodeInfo(nodeInfo)

	crawler := &Crawler{
		addrBook:  book,
		nodeInfo:  nodeInfo,
		nodeKey:   nodeKey,
		transport: transport,
		sw:        sw,
		logger:    logger,
	}

	return crawler, nil
}

func (c *Crawler) Crawl(ctx context.Context) error {
	nodeInfo := c.nodeInfo
	addr, err := p2p.NewNetAddressString(p2p.IDAddressString(nodeInfo.DefaultNodeID, nodeInfo.ListenAddr))
	if err != nil {
		return err
	}

	// Listen for incoming p2p
	if err := c.transport.Listen(*addr); err != nil {
		return err
	}
	defer c.transport.Close()

	// Start p2p switch
	if err = c.sw.Start(); err != nil {
		return err
	}

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

loop:
	for {
		select {
		case <-ticker.C:
			c.logger.Debug("", "peers", c.sw.Peers().List())
			c.addrBook.Save()
		case <-ctx.Done():
			c.logger.Info("Saving address book")
			c.addrBook.Save()
			c.logger.Info("Stopping p2p switch")
			if err := c.sw.Stop(); err != nil {
				return err
			}
			break loop
		}
	}

	return nil
}

func (c *Crawler) AddressBook() pex.AddrBook {
	return c.addrBook
}
