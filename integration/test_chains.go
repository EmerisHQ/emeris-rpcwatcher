package integration

import (
	"fmt"

	"github.com/allinbits/emeris-rpcwatcher/rpcwatcher/database"
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

func getInsertQueryValue(index string, chain testChain) string {
	value := `(` + index + `, true, '` + chain.chainID + `', '10s', 'logo url', '` + chain.chainID + `', '{`
	i := 0
	for k, v := range chain.channels {
		value = value + `"` + k + `": "` + v + `"`
		if i != len(chain.channels)-1 {
			value = value + `,`
		}
		i++
	}
	value = value + `}',
	'[
		{"display_name":"` + chain.accountInfo.displayDenom + `","name":"` + chain.accountInfo.denom + `","verified":true,"fetch_price":true,"fee_token":true,"fee_levels":{"low":1,"average":22,"high":42},"precision":6}
	]', 
	ARRAY['feeaddress'], 'genesis_hash', 
	'{"endpoint":"http://localhost:` + chain.rpcPort + `","chain_id":"` + chain.chainID + `","bech32_config":{"main_prefix":"main_prefix","prefix_account":"` + chain.accountInfo.prefix + `","prefix_validator":"prefix_validator",
	"prefix_consensus":"prefix_consensus","prefix_public":"prefix_public","prefix_operator":"prefix_operator"}}', 
	'm/44''/118''/0''/0/0')`
	return value
}

func getMigrations(chains []testChain) []string {
	var migrations []string
	migrations = append(migrations, database.CreateDB, database.CreateCNSTable)
	insertQuery := `INSERT INTO cns.chains 
	(
	id, enabled, chain_name, valid_block_thresh, logo, display_name, primary_channel, denoms, demeris_addresses, 
	genesis_hash, node_info, derivation_path
	) 
	VALUES `
	for i, chain := range chains {
		insertQuery = insertQuery + getInsertQueryValue(fmt.Sprintf("%d", i+1), chain)
		if i != len(chains)-1 {
			insertQuery = insertQuery + `,`
		}
	}

	migrations = append(migrations, insertQuery)
	return migrations
}
