package integration

import (
	"github.com/ory/dockertest/v3"
)

type testChain struct {
	chainID     string
	accountInfo accountInfo
	dockerfile  string
	binaryName  string
	rpcPort     string
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
	denom        string
	displayDenom string
}

const (
	// SEED1 is a mnenomic
	//nolint:lll
	SEED1 = "cake blossom buzz suspect image view round utility meat muffin humble club model latin similar glow draw useless kiwi snow laugh gossip roof public"
	// SEED2 is a mnemonic
	//nolint:lll
	SEED2 = "near little movie lady moon fuel abandon gasp click element muscle elbow taste indoor soft soccer like occur legend coin near random normal adapt"
)

var (
	gaiaTestChain = testChain{
		chainID: "cosmos-hub",
		accountInfo: accountInfo{
			keyname:      "validator",
			seed:         SEED1,
			address:      "cosmos1gclfxn8qyeytlupzjgzm6cmaxsdp7nlnzesjxa",
			prefix:       "cosmos",
			denom:        "uatom",
			displayDenom: "ATOM",
		},
		binaryName:  "gaiad",
		dockerfile:  "integration/setup/Dockerfile.gaiatest",
		testAddress: "cosmos12tkgplyat382h8eznp0ams3uz2gsukz7a9h76s",
	}

	akashTestChain = testChain{
		chainID: "akash",
		accountInfo: accountInfo{
			keyname:      "validator",
			seed:         SEED1,
			address:      "akash1gclfxn8qyeytlupzjgzm6cmaxsdp7nln0za4l8",
			prefix:       "akash",
			denom:        "uakt",
			displayDenom: "AKT",
		},
		binaryName:  "akash",
		dockerfile:  "integration/setup/Dockerfile.akashtest",
		testAddress: "akash12tkgplyat382h8eznp0ams3uz2gsukz7s76er2",
	}
)
