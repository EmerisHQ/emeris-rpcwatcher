#!/bin/sh

set -o errexit -o nounset

if [ -z "$1" ]; then
  echo "Need to input port..."
  exit 1
fi

IP1=$1
IP2=$2

rly cfg init

rly cfg show

curl http://$IP1:26657/status
curl http://$IP2:26657/status
