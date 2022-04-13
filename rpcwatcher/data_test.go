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
	defaultChannel = "channel-0"

	defaultPoolDenom = "pool96EF6EA6E5AC828ED87E8D07E7AE2A8180570ADD212117B2DA6F0B75D17A6295"

	defaultHeight = 123

	defaultPktSeq = "2"

	newPoolDenom = "pool96EF6EA6E5AC828ED87E8D07E7AE2A8180570ADD212117B2DA6F0B75D17A6294"

	testOwner = "cosmos1vaa40n5naka7mav3za6kx40jckx6aa4nqvvx8a"
)

var (
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
	data, err := ioutil.ReadFile("./testdata/ibc-transfer-receive-tx.json")
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
	data, err := ioutil.ReadFile("./testdata/ibc-transfer-transfer-tx-ack.json")
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
	data, err := ioutil.ReadFile("./testdata/ibc-transfer-timeout-tx.json")
	require.NoError(t, err)
	event := txJSONToResultEvent(t, data)
	txHashSlice, exists := event.Events["tx.hash"]
	require.True(t, exists, "hash not found in given timeout tx json")
	require.GreaterOrEqual(t, 1, len(txHashSlice))
	ibcTimeoutTxHash = txHashSlice[0]
	return event
}

func ibcTransferEvent(t *testing.T) coretypes.ResultEvent {
	data, err := ioutil.ReadFile("./testdata/ibc-transfer-transfer-tx.json")
	require.NoError(t, err)
	event := txJSONToResultEvent(t, data)
	txHashSlice, exists := event.Events["tx.hash"]
	require.True(t, exists, "hash not found in given ibc transfer tx json")
	require.GreaterOrEqual(t, 1, len(txHashSlice))
	ibcTransferTxHash = txHashSlice[0]
	return event
}

func nonIBCTransferEvent(t *testing.T, codeZero bool) coretypes.ResultEvent {
	data, err := ioutil.ReadFile("./testdata/non-ibc-transfer-tx.json")
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
	data, err := ioutil.ReadFile("./testdata/swap-tx.json")
	require.NoError(t, err)
	event := txJSONToResultEvent(t, data)
	txHashSlice, exists := event.Events["tx.hash"]
	require.True(t, exists, "hash not found in given swap tx json")
	require.GreaterOrEqual(t, 1, len(txHashSlice))
	swapTxHash = txHashSlice[0]
	return event
}

func createPoolEvent(t *testing.T, newDenom bool) coretypes.ResultEvent {
	data, err := ioutil.ReadFile("./testdata/tx-create-lp.json")
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
