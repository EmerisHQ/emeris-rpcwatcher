#!/bin/sh

set -o errexit -o nounset

if [ -z "$1" ]; then
  echo "Need to input port..."
  exit 1
fi

PORT=$1

rly cfg init

rly cfg show

curl $PORT/status