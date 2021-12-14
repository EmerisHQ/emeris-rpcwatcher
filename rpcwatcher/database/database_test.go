package database

import (
	"log"
	"os"
	"testing"

	cnsmodels "github.com/allinbits/demeris-backend-models/cns"
	dbutils "github.com/allinbits/emeris-utils/database"
	"github.com/cockroachdb/cockroach-go/v2/testserver"
	"github.com/stretchr/testify/require"
)

const (
	defaultChainName = "cosmoshub"
)

var (
	testDBMigrations = []string{
		`CREATE DATABASE IF NOT EXISTS cns`,
		`CREATE TABLE IF NOT EXISTS cns.chains (
		id serial unique primary key,
		enabled boolean default false,
		chain_name string not null,
		valid_block_thresh string not null,
		logo string not null,
		display_name string not null,
		primary_channel jsonb not null,
		denoms jsonb not null,
		demeris_addresses text[] not null,
		genesis_hash string not null,
		node_info jsonb not null,
		derivation_path string not null,
		unique(chain_name)
		)`,
		`
	INSERT INTO cns.chains 
		(
		id, enabled, chain_name, valid_block_thresh, logo, display_name, primary_channel, denoms, demeris_addresses, 
		genesis_hash, node_info, derivation_path
		) 
		VALUES (
			1, true, '` + defaultChainName + `', '10s', 'logo url', 'Cosmos Hub', 
			'{"cosmos-hub": "channel-0", "akash": "channel-1"}',
			'[
				{"display_name":"USD","name":"uusd","verified":true,"fee_token":true,"fetch_price":true,"fee_levels":{"low":1,"average":22,"high":42},"precision":6},
				{"display_name":"ATOM","name":"uatom","verified":true,"fetch_price":true,"fee_token":true,"fee_levels":{"low":1,"average":22,"high":42},"precision":6}
			]', 
			ARRAY['feeaddress'], 'genesis_hash', 
			'{"endpoint":"endpoint","chain_id":"chainid","bech32_config":{"main_prefix":"main_prefix","prefix_account":"prefix_account","prefix_validator":"prefix_validator",
			"prefix_consensus":"prefix_consensus","prefix_public":"prefix_public","prefix_operator":"prefix_operator"}}', 
			'm/44''/118''/0''/0/0')
	`,
	}

	// global variable for tests
	dbInstance *Instance
)

// TODO: remove duplicate setupDB function in watcher_test.go
// once https://github.com/allinbits/emeris-rpcwatcher/pull/5 is merged
func TestMain(m *testing.M) {
	// setup test DB
	var ts testserver.TestServer
	ts, dbInstance = setupDB()
	defer ts.Stop()

	code := m.Run()
	os.Exit(code)
}

func setupDB() (testserver.TestServer, *Instance) {
	// start new cockroachDB test server
	ts, err := testserver.NewTestServer()
	checkNoError(err)

	err = ts.WaitForInit()
	checkNoError(err)

	// create new instance of db
	i, err := New(ts.PGURL().String())
	checkNoError(err)

	// create and insert data into db
	err = dbutils.RunMigrations(ts.PGURL().String(), testDBMigrations)
	checkNoError(err)

	return ts, i
}

func checkNoError(err error) {
	if err != nil {
		log.Fatalf("got error: %s", err)
	}
}

func TestChains(t *testing.T) {
	chains, err := dbInstance.Chains()
	require.NoError(t, err)
	require.Len(t, chains, 1)
	require.Equal(t, defaultChainName, chains[0].ChainName)
}

func TestChain(t *testing.T) {
	tests := []struct {
		name      string
		chainName string
		expErr    bool
	}{
		{
			"Get chain details with invalid chain name",
			"invalid",
			true,
		},
		{
			"Get chain details with valid chain name",
			defaultChainName,
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chain, err := dbInstance.Chain(tt.chainName)
			if tt.expErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.chainName, chain.ChainName)
			}
		})
	}
}

func TestGetCounterParty(t *testing.T) {
	tests := []struct {
		name      string
		chainName string
		channel   string
		expected  []cnsmodels.ChannelQuery
		expErr    bool
	}{
		{
			"Get counterparty details of invalid chain name",
			"invalid",
			"",
			[]cnsmodels.ChannelQuery{},
			true,
		},
		{
			"Get counterparty details with invalid channel name",
			defaultChainName,
			"test-channel",
			[]cnsmodels.ChannelQuery{},
			true,
		},
		{
			"Get counterparty details with valid chain and channel names",
			defaultChainName,
			"channel-1",
			[]cnsmodels.ChannelQuery{
				{
					ChainName:    defaultChainName,
					Counterparty: "akash",
					ChannelName:  "channel-1",
				},
			},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			counterparty, err := dbInstance.GetCounterParty(tt.chainName, tt.channel)
			if tt.expErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, counterparty)
			}
		})
	}
}

func TestUpdateDenoms(t *testing.T) {
	tests := []struct {
		name      string
		executeFn func(*testing.T, cnsmodels.Chain) cnsmodels.Chain
		expErr    bool
		checkLen  bool
	}{
		{
			"Update denoms of invalid chain",
			func(t *testing.T, _ cnsmodels.Chain) cnsmodels.Chain {
				return cnsmodels.Chain{ChainName: "invalid"}
			},
			true,
			false,
		},
		{
			"Update denoms of valid chain without any changes",
			func(t *testing.T, chain cnsmodels.Chain) cnsmodels.Chain {
				return chain
			},
			false,
			true,
		},
		{
			"Update denoms of valid chain with adding new denom",
			func(t *testing.T, chain cnsmodels.Chain) cnsmodels.Chain {
				chain.Denoms = append(chain.Denoms, cnsmodels.Denom{
					Name:        "ustake",
					DisplayName: "Stake",
				})
				return chain
			},
			false,
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getChain, err := dbInstance.Chain(defaultChainName)
			require.NoError(t, err)
			updatedChain := tt.executeFn(t, getChain)
			err = dbInstance.UpdateDenoms(updatedChain)
			if tt.expErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			if tt.checkLen {
				updatedChainFromDB, err := dbInstance.Chain(defaultChainName)
				require.NoError(t, err)
				require.Len(t, updatedChainFromDB.Denoms, len(updatedChain.Denoms))
			}
		})
	}
}
