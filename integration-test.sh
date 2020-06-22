#!/usr/bin/env bash
set -e

DIR=$PWD

curl -vsS https://raw.githubusercontent.com/hyperledger/fabric/master/scripts/bootstrap.sh | bash
cd ./fabric-samples/first-network
echo y | ./byfn.sh up
cp -r crypto-config "$DIR" && cd "$DIR"
go build
export stupid_loglevel=\"debug\"
./stupid config.json 100
