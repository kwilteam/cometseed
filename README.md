# CometSeed

This tool runs a seeder for a CometBFT blockchain, crawling the network and
serving peer addresses to incoming peers.  It is inspired by TinySeed from
notional-labs.

## Use

### Build

There are no special build procedures.

```sh
git clone https://github.com/kwilteam/cometseed
go build
```

or

```sh
go install -v github.com/kwilteam/cometseed@latest
```

### Run

Two required settings are exposed with command line flags or environment variables:

```plain
Usage of ./cometseed:
  -chain-id string
    	chain ID
  -seeds string
    	seed nodes
```

The chain ID given by `-chain-id` should match the ID reported by the nodes it connects to.

The nodes specified by `-seeds` are bootstrap nodes used on the first startup to begin crawling the network with peer exchange.

To start using command line flags:

```sh
./cometseed -chain-id chain-id-c2315x -seeds "beefbeefbeefbeefbeefbeef@127.0.0.1:26656"
```

Using environment variables:

```sh
CHAIN=chain-id-c2315x SEEDS=beefbeefbeefbeefbeefbeef@127.0.0.1:26656 ./cometseed
```

`cometseed` will then be listening on 26656.

Alternatively, edit `~/.cometseed/config.toml`. The config file also exposes a few advanced settings, including `listen`, which specifies the TCP address to listen on.  These options will become flags in the future.

### Artifacts

In `~/.cometseed`, there will be `addrbook.json` and `node_key.json`.

The identity of `cometseed` in the p2p network is randomly generated on first startup and stored in `node_key.json`. It does not need to be kept.

The list of known peers is stored in `addrbook.json`. After having found an inserted at least one node, the `-seeds` setting is no longer needed when starting `cometseed`

## License

[MIT](./LICENSE)
