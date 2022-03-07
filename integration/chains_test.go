package integration

import (
	"github.com/ory/dockertest/v3"
)

type testChain struct {
	chainID     string
	accountInfo accountInfo
	denoms      []denomInfo
	dockerfile  string
	binaryName  string
	nodeAddress string
	resource    *dockertest.Resource
	channels    map[string]string
	// dummy second address to test txs
	testAddress string
}

type accountInfo struct {
	keyname      string
	seed         string
	address      string
	prefix       string
	primaryDenom string
}

type denomInfo struct {
	denom        string
	displayDenom string
}

const (
	// mnemonic is a mnenomic
	mnemonic = "cake blossom buzz suspect image view round utility meat muffin humble club model latin similar glow draw useless kiwi snow laugh gossip roof public"

	defaultRPCPort  = "26657"
	defaultGRPCPort = "9090"
)

var (
	gaiaTestChain = testChain{
		chainID: "cosmos-hub",
		accountInfo: accountInfo{
			keyname:      "validator",
			seed:         mnemonic,
			address:      "cosmos1gclfxn8qyeytlupzjgzm6cmaxsdp7nlnzesjxa",
			prefix:       "cosmos",
			primaryDenom: "uatom",
		},
		binaryName:  "gaiad",
		dockerfile:  "integration/setup/Dockerfile.gaiatest",
		testAddress: "cosmos12tkgplyat382h8eznp0ams3uz2gsukz7a9h76s",
		denoms: []denomInfo{
			{
				denom:        "uatom",
				displayDenom: "ATOM",
			},
			{
				denom:        "samoleans",
				displayDenom: "LEANS",
			},
			{
				denom:        "samoleans2",
				displayDenom: "LEANS2",
			},
		},
	}

	akashTestChain = testChain{
		chainID: "akash",
		accountInfo: accountInfo{
			keyname:      "validator",
			seed:         mnemonic,
			address:      "akash1gclfxn8qyeytlupzjgzm6cmaxsdp7nln0za4l8",
			prefix:       "akash",
			primaryDenom: "uakt",
		},
		binaryName:  "akash",
		dockerfile:  "integration/setup/Dockerfile.akashtest",
		testAddress: "akash12tkgplyat382h8eznp0ams3uz2gsukz7s76er2",
		denoms: []denomInfo{
			{
				denom:        "uakt",
				displayDenom: "uatom",
			},
			{
				denom:        "samoleans",
				displayDenom: "LEANS",
			},
		},
	}
)
