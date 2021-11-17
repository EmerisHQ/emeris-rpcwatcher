package rpcwatcher

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/allinbits/emeris-rpcwatcher/rpcwatcher/database"
	dbutils "github.com/allinbits/emeris-rpcwatcher/utils/database"
	"github.com/allinbits/emeris-rpcwatcher/utils/store"
	"github.com/cockroachdb/cockroach-go/v2/testserver"
	"github.com/stretchr/testify/require"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
	"go.uber.org/zap"
)

var (
	s                *store.Store
	mr               *miniredis.Miniredis
	watcherInstance  *Watcher
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
				1, true, 'cosmos-hub', '10s', 'logo url', 'Cosmos Hub', 
				'{"cn1": "cn1", "cn2": "cn2"}',
				'[
					{"display_name":"STAKE","name":"stake","verified":true,"fee_token":true,"fetch_price":true,"fee_levels":{"low":1,"average":22,"high":42},"precision":6},
					{"display_name":"ATOM","name":"uatom","verified":true,"fetch_price":true,"fee_token":true,"fee_levels":{"low":1,"average":22,"high":42},"precision":6}
				]', 
				ARRAY['feeaddress'], 'genesis_hash', 
				'{"endpoint":"endpoint","chain_id":"chainid","bech32_config":{"main_prefix":"main_prefix","prefix_account":"prefix_account","prefix_validator":"prefix_validator",
				"prefix_consensus":"prefix_consensus","prefix_public":"prefix_public","prefix_operator":"prefix_operator"}}', 
				'm/44''/118''/0''/0/0')
		`,
		`
		INSERT INTO cns.chains 
			(
			id, enabled, chain_name, valid_block_thresh, logo, display_name, primary_channel, denoms, demeris_addresses, 
			genesis_hash, node_info, derivation_path
			) 
			VALUES (
				2, true, 'akash', '10s', 'logo url', 'Akash Network', 
				'{"cn3": "cn3", "cn4": "cn4"}',
				'[
					{"display_name":"AKT","name":"uakt","verified":true,"fetch_price":true,"fee_token":true,"fee_levels":{"low":1,"average":22,"high":42},"precision":6}
				]', 
				ARRAY['feeaddress2'], 'genesis_hash_2', 
				'{"endpoint":"endpoint2","chain_id":"chainid2","bech32_config":{"main_prefix":"main_prefix","prefix_account":"prefix_account","prefix_validator":"prefix_validator",
				"prefix_consensus":"prefix_consensus","prefix_public":"prefix_public","prefix_operator":"prefix_operator"}}', 
				'm/44''/118''/0''/0/0')
		`,
	}
)

// TODO: use mocks for data in db
func TestMain(m *testing.M) {
	// start new cockroachDB test server
	ts, err := testserver.NewTestServer()
	if err != nil {
		log.Fatalf("got error: %s when running new test server", err)
	}
	err = ts.WaitForInit()
	if err != nil {
		log.Fatalf("got error: %s when waiting for init of test server", err)
	}
	defer ts.Stop()

	// create new instance of db
	i, err := database.New(ts.PGURL().String())
	if err != nil {
		log.Fatalf("got error: %s when creating db instance", err)
	}

	// create and insert data into db
	err = dbutils.RunMigrations(ts.PGURL().String(), testDBMigrations)
	if err != nil {
		log.Fatalf("got error: %s when running migrations", err)
	}

	c, err := i.Chains()
	if err != nil {
		log.Fatalf("got error: %s when getting chains", err)
	}
	fmt.Println(c)
	l := zap.NewNop().Sugar()

	// setup test store
	mr, s = store.SetupTestStore()
	watcherInstance = &Watcher{
		l:     l,
		d:     i,
		store: s,
		Name:  "cosmos-hub",
	}
	code := m.Run()
	defer mr.Close()
	os.Exit(code)
}

const dummyjson = `
{"query":"tm.event='Tx'","data":{},
"events":{"delegate.amount":["20000000"],"delegate.validator":["akashvaloper1ep8nr7d383ng7pndvl2u9j6c9tdkwyr6dvtn2n"],"message.action":["delegate"],"message.module":["staking"],"message.sender":["akash1vaa40n5naka7mav3za6kx40jckx6aa4nqvvx8a","akash1jv65s3grqf6v6jl3dp4t6c9t9rk99cd82yfms9","akash1vaa40n5naka7mav3za6kx40jckx6aa4nqvvx8a"],"tm.event":["Tx"],"transfer.amount":["500uakt","982uakt"],"transfer.recipient":["akash17xpfvakm2amg962yls6f84z3kell8c5lazw8j8","akash1vaa40n5naka7mav3za6kx40jckx6aa4nqvvx8a"],"transfer.sender":["akash1vaa40n5naka7mav3za6kx40jckx6aa4nqvvx8a","akash1jv65s3grqf6v6jl3dp4t6c9t9rk99cd82yfms9"],
"tx.hash":["D88B758F52DD059958B55B5874105BDF421F0685BA7EB1ACBC7D04337D6466E0"],"tx.height":["3502864"]}}`

const dummyData = `{"height":3502864,"tx":"","result":{"data":"CgoKCGRlbGVnYXRl","log":"[]","code":0, "gas_wanted":200000,"gas_used":149189,"events":[]}}`

func TestHandleMessage(t *testing.T) {
	defer store.ResetTestStore(mr, s)
	var c coretypes.ResultEvent
	err := json.Unmarshal([]byte(dummyjson), &c)
	require.NoError(t, err)
	var d types.EventDataTx
	err = json.Unmarshal([]byte(dummyData), &d)
	c.Data = d
	t.Log(c.Data)
	require.NoError(t, err)
	s.CreateTicket("cosmos-hub", "D88B758F52DD059958B55B5874105BDF421F0685BA7EB1ACBC7D04337D6466E0", "akash1vaa40n5naka7mav3za6kx40jckx6aa4nqvvx8a")
	HandleMessage(watcherInstance, c)
	ticket, err := s.Get(store.GetKey("cosmos-hub", "D88B758F52DD059958B55B5874105BDF421F0685BA7EB1ACBC7D04337D6466E0"))
	require.NoError(t, err)
	require.Equal(t, "complete", ticket.Status)
}
