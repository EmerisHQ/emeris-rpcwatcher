#!/bin/sh

set -o errexit -o nounset

if [ -z "$1" ]; then
  echo "Need to input chain id..."
  exit 1
fi

if [ -z "$2" ]; then
  echo "Need to input mnemonic seeds.."
  exit 1
fi

CHAINID=$1
SEEDS=$2
DENOM=$3

# Build genesis file incl account for passed address
coins="10000000000$DENOM,100000000000samoleans"
gaiad init --chain-id $CHAINID $CHAINID
echo "$SEEDS" | gaiad keys add validator --keyring-backend="test" --recover
gaiad add-genesis-account $(gaiad keys show validator -a --keyring-backend="test") $coins
gaiad gentx validator 5000000000$DENOM --keyring-backend="test" --chain-id $CHAINID
gaiad collect-gentxs

# Set proper defaults and change ports
sed -i "s/stake/$DENOM/g" ~/.gaia/config/genesis.json
sed -i 's#"tcp://127.0.0.1:26657"#"tcp://0.0.0.0:26657"#g' ~/.gaia/config/config.toml
sed -i 's/timeout_commit = "5s"/timeout_commit = "1s"/g' ~/.gaia/config/config.toml
sed -i 's/timeout_propose = "3s"/timeout_propose = "1s"/g' ~/.gaia/config/config.toml
sed -i 's/index_all_keys = false/index_all_keys = true/g' ~/.gaia/config/config.toml

# Start the gaia
gaiad start --pruning=nothing
