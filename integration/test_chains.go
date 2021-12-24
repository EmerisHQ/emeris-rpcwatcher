package integration

import "github.com/ory/dockertest/v3"

type testChain struct {
	chainID     string
	accountInfo accountInfo
	dockerfile  string
	binaryName  string
	rpcPort     string
	resource    *dockertest.Resource
}

type accountInfo struct {
	seed     string
	address  string
	prefix   string
	denom    string
	channels map[string]string
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
			seed:    SEED1,
			address: "cosmos1gclfxn8qyeytlupzjgzm6cmaxsdp7nlnzesjxa",
			prefix:  "cosmos",
			denom:   "uatom",
		},
		binaryName: "gaiad",
		dockerfile: "integration/setup/Dockerfile.gaiatest",
	}

	akashTestChain = testChain{
		chainID: "akash",
		accountInfo: accountInfo{
			seed:    SEED1,
			address: "akash1gclfxn8qyeytlupzjgzm6cmaxsdp7nln0za4l8",
			prefix:  "akash",
			denom:   "uakt",
		},
		binaryName: "akash",
		dockerfile: "integration/setup/Dockerfile.akashtest",
	}
)
