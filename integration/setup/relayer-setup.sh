#!/bin/sh

set -o errexit -o nounset

PTH=$1
CHAINID1=$2
DENOM1=$3
PREFIX1=$4
SEEDS1=$5
RPC1=$6
CHAINID2=$7
DENOM2=$8
PREFIX2=$9
SEEDS2=${10}
RPC2=${11}

export RLYKEY=key
export ACCOUNT_PREFIX=cosmos
export RELAYER_DAEMON=rly

CHAINS=2

$RELAYER_DAEMON config init

mkdir -p testdata

echo "------ create json files ---------"

echo "{\"key\":\"$RLYKEY\",\"chain-id\":\"$CHAINID1\",\"rpc-addr\":\"$RPC1\",\"account-prefix\":\"$PREFIX1\",\"gas-adjustment\": 1.5,\"gas\":200000,\"gas-prices\":\"0$DENOM1\",\"default-denom\":\"$DENOM1\",\"trusting-period\":\"330h\"}" > testdata/$CHAINID1.json
echo "{\"key\":\"$RLYKEY\",\"chain-id\":\"$CHAINID2\",\"rpc-addr\":\"$RPC2\",\"account-prefix\":\"$PREFIX2\",\"gas-adjustment\": 1.5,\"gas\":200000,\"gas-prices\":\"0$DENOM2\",\"default-denom\":\"$DENOM2\",\"trusting-period\":\"330h\"}" > testdata/$CHAINID2.json

echo "------- add chains-------------"
$RELAYER_DAEMON chains add -f testdata/$CHAINID1.json
$RELAYER_DAEMON chains add -f testdata/$CHAINID2.json

echo "---------restore keys with existing seeds--------"
$RELAYER_DAEMON keys restore $CHAINID1 "$RLYKEY" "$SEEDS1"
export SRC=$CHAINID1
$RELAYER_DAEMON keys restore $CHAINID2 "$RLYKEY" "$SEEDS2"
export DST=$CHAINID2
echo "----------create a light client----------"
$RELAYER_DAEMON light init $CHAINID1 -f
$RELAYER_DAEMON light init $CHAINID2 -f

echo

echo "---------create a testdata/$PTH.json------------"
echo "{\"src\":{\"chain-id\":\"$SRC\",\"port-id\":\"transfer\",\"order\":\"unordered\",\"version\":\"ics20-1\"},\"dst\":{\"chain-id\":\"$DST\",\"port-id\":\"transfer\",\"order\":\"unordered\",\"version\":\"ics20-1\"},\"strategy\":{\"type\":\"naive\"}}" > testdata/$PTH.json

echo "------ add a path between $SRC and $DST ----"
$RELAYER_DAEMON pth add $SRC $DST $PTH -f testdata/$PTH.json

echo "-----show keys----------"
$RELAYER_DAEMON keys show $SRC
$RELAYER_DAEMON keys show $DST

echo

echo "--------link path--------------"
$RELAYER_DAEMON tx link $PTH --override

sleep 2s

echo "-------start relayer--------"
$RELAYER_DAEMON start $PTH
