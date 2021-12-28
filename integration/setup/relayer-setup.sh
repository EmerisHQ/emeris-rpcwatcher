#!/usr/bin/bash

set -o errexit -o nounset

TEMPDIR=$1
RLYDIR=$2
PTH=$3
CHAINID1=$4
DENOM1=$5
PREFIX1=$6
SEEDS1=$7
RPCPORT1=$8
CHAINID2=$9
DENOM2=${10}
PREFIX2=${11}
SEEDS2=${12}
RPCPORT2=${13}

cd $TEMPDIR

export RELAYER_HOME=$TEMPDIR/$RLYDIR
export GH_URL=https://github.com/cosmos/relayer.git
export CHAIN_VERSION=492378a804a6d533d6635ab6eabe39d8bfab2c57
export RLYKEY=key
export DOMAIN=localhost
export ACCOUNT_PREFIX=cosmos
export RELAYER_DAEMON=./build/rly

CHAINS=2

# check relayer code exists
if [ -d "relayer" ]; then
    cd relayer
else
    echo "--------- Install relayer ---------"
    git clone $GH_URL && cd relayer
fi
git fetch && git checkout $CHAIN_VERSION
make build

echo "----- remove relayer home dir if already exists -------"
rm -rf $RELAYER_HOME

$RELAYER_DAEMON config init --home $RELAYER_HOME

mkdir -p testdata

echo "------ create json files ---------"

echo "{\"key\":\"$RLYKEY\",\"chain-id\":\"$CHAINID1\",\"rpc-addr\":\"http://$DOMAIN:$RPCPORT1\",\"account-prefix\":\"$PREFIX1\",\"gas-adjustment\": 1.5,\"gas\":200000,\"gas-prices\":\"0$DENOM1\",\"default-denom\":\"$DENOM1\",\"trusting-period\":\"330h\"}" > testdata/$CHAINID1.json
echo "{\"key\":\"$RLYKEY\",\"chain-id\":\"$CHAINID2\",\"rpc-addr\":\"http://$DOMAIN:$RPCPORT2\",\"account-prefix\":\"$PREFIX2\",\"gas-adjustment\": 1.5,\"gas\":200000,\"gas-prices\":\"0$DENOM2\",\"default-denom\":\"$DENOM2\",\"trusting-period\":\"330h\"}" > testdata/$CHAINID2.json

echo "------- add chains-------------"
$RELAYER_DAEMON chains add -f testdata/$CHAINID1.json --home $RELAYER_HOME
$RELAYER_DAEMON chains add -f testdata/$CHAINID2.json --home $RELAYER_HOME

echo "---------restore keys with existing seeds--------"
$RELAYER_DAEMON keys restore $CHAINID1 "$RLYKEY" "$SEEDS1" --home $RELAYER_HOME
export SRC=$CHAINID1
$RELAYER_DAEMON keys restore $CHAINID2 "$RLYKEY" "$SEEDS2" --home $RELAYER_HOME
export DST=$CHAINID2
echo "----------create a light client----------"
$RELAYER_DAEMON light init $CHAINID1 -f --home $RELAYER_HOME
$RELAYER_DAEMON light init $CHAINID2 -f --home $RELAYER_HOME

echo

echo "---------create a testdata/$PTH.json------------"
echo "{\"src\":{\"chain-id\":\"$SRC\",\"port-id\":\"transfer\",\"order\":\"unordered\",\"version\":\"ics20-1\"},\"dst\":{\"chain-id\":\"$DST\",\"port-id\":\"transfer\",\"order\":\"unordered\",\"version\":\"ics20-1\"},\"strategy\":{\"type\":\"naive\"}}" > testdata/$PTH.json

echo "------ add a path between $SRC and $DST ----"
$RELAYER_DAEMON pth add $SRC $DST $PTH -f testdata/$PTH.json --home $RELAYER_HOME

echo "-----show keys----------"
$RELAYER_DAEMON keys show $SRC --home $RELAYER_HOME
$RELAYER_DAEMON keys show $DST --home $RELAYER_HOME

echo

echo "--------link path--------------"
$RELAYER_DAEMON tx link $PTH --override --home $RELAYER_HOME

sleep 2s

echo "-------start relayer--------"
$RELAYER_DAEMON start $PTH --home $RELAYER_HOME >/dev/null 2>&1 &

echo "------Relayer setup done successfully-------"
