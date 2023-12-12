# CometSeed

This tool runs a seeder for a CometBFT blockchain, crawling the network and
serving peer addresses to incoming peers.  It is inspired by TinySeed from
notional-labs.

## Use

```bash
git clone https://github.com/kwilteam/cometseed
go build
CHAIN=chain-id-c2315x SEEDS=beefbeefbeefbeefbeefbeef@127.0.0.1:26656 ./cometseed
# OR
./cometseed -chain-id chain-id-c2315x -seeds "beefbeefbeefbeefbeefbeef@127.0.0.1:26656"
# OR edit ~/.cometseed/config.toml
```

## License

[Blue Oak Model License 1.0.0](https://blueoakcouncil.org/license/1.0.0)
