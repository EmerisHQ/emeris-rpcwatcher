package database

import (
	"os"
	"testing"

	"github.com/cockroachdb/cockroach-go/v2/testserver"
	cnsmodels "github.com/emerishq/demeris-backend-models/cns"
	"github.com/stretchr/testify/require"
)

// global variable for tests
var dbInstance *Instance

// TODO: remove duplicate setupDB function in watcher_test.go
// once https://github.com/emerishq/emeris-rpcwatcher/pull/5 is merged
func TestMain(m *testing.M) {
	// setup test DB
	var ts testserver.TestServer
	ts, dbInstance = SetupTestDB(TestDBMigrations)
	defer ts.Stop()

	code := m.Run()
	os.Exit(code)
}

func TestChains(t *testing.T) {
	chains, err := dbInstance.Chains()
	require.NoError(t, err)
	require.Len(t, chains, 1)
	require.Equal(t, TestChainName, chains[0].ChainName)
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
			TestChainName,
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
			TestChainName,
			"test-channel",
			[]cnsmodels.ChannelQuery{},
			true,
		},
		{
			"Get counterparty details with valid chain and channel names",
			TestChainName,
			"channel-1",
			[]cnsmodels.ChannelQuery{
				{
					ChainName:    TestChainName,
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
			getChain, err := dbInstance.Chain(TestChainName)
			require.NoError(t, err)
			updatedChain := tt.executeFn(t, getChain)
			err = dbInstance.UpdateDenoms(updatedChain)
			if tt.expErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			if tt.checkLen {
				updatedChainFromDB, err := dbInstance.Chain(TestChainName)
				require.NoError(t, err)
				require.Len(t, updatedChainFromDB.Denoms, len(updatedChain.Denoms))
			}
		})
	}
}
