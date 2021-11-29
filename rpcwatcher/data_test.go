package rpcwatcher

import (
	"encoding/json"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
)

const (
	defaultChainName = "cosmos-hub"

	defaultChannel = "channel-0"

	defaultPoolDenom = "pool96EF6EA6E5AC828ED87E8D07E7AE2A8180570ADD212117B2DA6F0B75D17A6295"

	defaultHeight = 123

	defaultPktSeq = "2"

	newPoolDenom = "pool96EF6EA6E5AC828ED87E8D07E7AE2A8180570ADD212117B2DA6F0B75D17A6294"

	testOwner = "cosmos1vaa40n5naka7mav3za6kx40jckx6aa4nqvvx8a"
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
				1, true, 'cosmos-hub', '10s', 'logo url', 'Cosmos Hub', 
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

	nonIBCTransferTxHash string

	ibcTransferTxHash string

	ibcAckTxHash string

	ibcReceiveTxHash string

	ibcTimeoutTxHash string

	createPoolTxHash string

	swapTxHash string
)

type TxJSON struct {
	JSONRPC string                `json:"jsonrpc"`
	ID      int64                 `json:"id"`
	Result  coretypes.ResultEvent `json:"result"`
}

func txJSONToResultEvent(t *testing.T, data []byte) coretypes.ResultEvent {
	var b TxJSON
	require.NoError(t, json.Unmarshal(data, &b))
	result := b.Result
	out, err := json.Marshal(result.Data)
	require.NoError(t, err)
	var d types.EventDataTx
	require.NoError(t, json.Unmarshal(out, &d))
	result.Data = d
	return result
}

func ibcReceivePacketEvent(t *testing.T, ackSuccess bool) coretypes.ResultEvent {
	data, err := ioutil.ReadFile("ibc-transfer-receive-tx.json")
	require.NoError(t, err)
	event := txJSONToResultEvent(t, data)
	if !ackSuccess {
		// modify write acknowledgement packet ack success to false
		event.Events["write_acknowledgement.packet_ack"] = []string{"{\"result\":\"AO==\"}"}
	}
	txHashSlice, exists := event.Events["tx.hash"]
	require.True(t, exists, "hash not found in given receive tx json")
	require.GreaterOrEqual(t, 1, len(txHashSlice))
	ibcReceiveTxHash = txHashSlice[0]
	return event
}

func ibcAckTxEvent(t *testing.T, withErrorField bool) coretypes.ResultEvent {
	data, err := ioutil.ReadFile("ibc-transfer-transfer-tx-ack.json")
	require.NoError(t, err)
	event := txJSONToResultEvent(t, data)
	if withErrorField {
		// add fungible token packet error value
		event.Events["fungible_token_packet.error"] = []string{"\u0001"}
	}
	txHashSlice, exists := event.Events["tx.hash"]
	require.True(t, exists, "hash not found in given ack tx json")
	require.GreaterOrEqual(t, 1, len(txHashSlice))
	ibcAckTxHash = txHashSlice[0]
	return event
}

func ibcTimeoutEvent(t *testing.T) coretypes.ResultEvent {
	data, err := ioutil.ReadFile("ibc-transfer-timeout-tx.json")
	require.NoError(t, err)
	event := txJSONToResultEvent(t, data)
	txHashSlice, exists := event.Events["tx.hash"]
	require.True(t, exists, "hash not found in given timeout tx json")
	require.GreaterOrEqual(t, 1, len(txHashSlice))
	ibcTimeoutTxHash = txHashSlice[0]
	return event
}

func ibcTransferEvent(t *testing.T) coretypes.ResultEvent {
	data, err := ioutil.ReadFile("ibc-transfer-transfer-tx.json")
	require.NoError(t, err)
	event := txJSONToResultEvent(t, data)
	txHashSlice, exists := event.Events["tx.hash"]
	require.True(t, exists, "hash not found in given ibc transfer tx json")
	require.GreaterOrEqual(t, 1, len(txHashSlice))
	ibcTransferTxHash = txHashSlice[0]
	return event
}

func nonIBCTransferEvent(t *testing.T, codeZero bool) coretypes.ResultEvent {
	data, err := ioutil.ReadFile("non-ibc-transfer-tx.json")
	require.NoError(t, err)
	event := txJSONToResultEvent(t, data)
	if !codeZero {
		// modify result code to non-zero value
		eventTx := event.Data.(types.EventDataTx)
		eventTx.Result.Code = 19
		event.Data = eventTx
	}
	txHashSlice, exists := event.Events["tx.hash"]
	require.True(t, exists, "hash not found in given non ibc tx json")
	require.GreaterOrEqual(t, 1, len(txHashSlice))
	nonIBCTransferTxHash = txHashSlice[0]
	return event
}

func swapTransactionEvent(t *testing.T) coretypes.ResultEvent {
	data, err := ioutil.ReadFile("swap-tx.json")
	require.NoError(t, err)
	event := txJSONToResultEvent(t, data)
	txHashSlice, exists := event.Events["tx.hash"]
	require.True(t, exists, "hash not found in given swap tx json")
	require.GreaterOrEqual(t, 1, len(txHashSlice))
	swapTxHash = txHashSlice[0]
	return event
}

func createPoolEvent(t *testing.T, newDenom bool) coretypes.ResultEvent {
	data, err := ioutil.ReadFile("tx-create-lp.json")
	require.NoError(t, err)
	event := txJSONToResultEvent(t, data)
	if newDenom {
		event.Events["create_pool.pool_coin_denom"] = []string{newPoolDenom}
	}
	txHashSlice, exists := event.Events["tx.hash"]
	require.True(t, exists, "hash not found in given create pool tx json")
	require.GreaterOrEqual(t, 1, len(txHashSlice))
	createPoolTxHash = txHashSlice[0]
	return event
}
