package rpcwatcher

import (
	"log"
	"os"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/allinbits/emeris-rpcwatcher/rpcwatcher/database"
	dbutils "github.com/allinbits/emeris-utils/database"
	"github.com/allinbits/emeris-utils/logging"
	"github.com/allinbits/emeris-utils/store"
	"github.com/cockroachdb/cockroach-go/v2/testserver"
	"github.com/stretchr/testify/require"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	"go.uber.org/zap"
)

// global variables for tests
var (
	s          *store.Store
	dbInstance *database.Instance
	mr         *miniredis.Miniredis
	logger     *zap.SugaredLogger
)

// TODO: use mocks for data in db
func TestMain(m *testing.M) {
	// setup test DB
	var ts testserver.TestServer
	ts, dbInstance = setupDB()
	defer ts.Stop()

	// logger
	logger = logging.New(logging.LoggingConfig{
		LogPath: "",
		Debug:   true,
	})

	// setup test store
	mr, s = store.SetupTestStore()
	code := m.Run()
	defer mr.Close()
	os.Exit(code)
}

func setupDB() (testserver.TestServer, *database.Instance) {
	// start new cockroachDB test server
	ts, err := testserver.NewTestServer()
	checkNoError(err)

	err = ts.WaitForInit()
	checkNoError(err)

	// create new instance of db
	i, err := database.New(ts.PGURL().String())
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

func TestHandleMessage(t *testing.T) {
	defer store.ResetTestStore(mr, s)

	tests := []struct {
		name       string
		event      coretypes.ResultEvent
		logger     *zap.SugaredLogger
		txHash     string
		expStatus  string
		validateFn func(*testing.T, *Watcher, coretypes.ResultEvent, string)
	}{
		{
			"Handle message without logger",
			nonIBCTransferEvent(t, true),
			nil,
			nonIBCTransferTxHash,
			"",
			func(t *testing.T, w *Watcher, data coretypes.ResultEvent, _ string) {
				require.Panics(t, func() { HandleMessage(w, data) })
			},
		},
		{
			"Handle non IBC transaction",
			nonIBCTransferEvent(t, true),
			logger,
			nonIBCTransferTxHash,
			"complete",
			func(t *testing.T, w *Watcher, data coretypes.ResultEvent, _ string) {
				t.Log("Non IBC....", data)
				HandleMessage(w, data)
			},
		},
		{
			"Handle failed non IBC transaction",
			nonIBCTransferEvent(t, false),
			logger,
			nonIBCTransferTxHash,
			"failed",
			func(t *testing.T, w *Watcher, data coretypes.ResultEvent, _ string) {
				HandleMessage(w, data)
			},
		},
		{
			"Handle create LP transaction",
			createPoolEvent(t, false),
			logger,
			createPoolTxHash,
			"complete",
			func(t *testing.T, w *Watcher, data coretypes.ResultEvent, _ string) {
				HandleMessage(w, data)
				checkDenomExists(t, w, defaultPoolDenom, true)
			},
		},
		{
			"Handle ibc transfer",
			ibcTransferEvent(t),
			logger,
			ibcTransferTxHash,
			"transit",
			func(t *testing.T, w *Watcher, data coretypes.ResultEvent, _ string) {
				HandleMessage(w, data)
			},
		},
		{
			"Handle IBC receive packet transaction",
			ibcReceivePacketEvent(t, true),
			logger,
			ibcReceiveTxHash,
			"IBC_receive_success",
			func(t *testing.T, w *Watcher, data coretypes.ResultEvent, key string) {
				checkAndSetInTransit(t, data, w, ibcReceiveTxHash, "recv_packet", key)
				HandleMessage(w, data)
			},
		},
		{
			"Handle IBC acknowledge packet transaction",
			ibcAckTxEvent(t, true),
			logger,
			ibcAckTxHash,
			"Tokens_unlocked_ack",
			func(t *testing.T, w *Watcher, data coretypes.ResultEvent, key string) {
				checkAndSetInTransit(t, data, w, ibcAckTxHash, "acknowledge_packet", key)
				HandleMessage(w, data)
			},
		},
		{
			"Handle IBC timeout packet transaction",
			ibcTimeoutEvent(t),
			logger,
			ibcTimeoutTxHash,
			"Tokens_unlocked_timeout",
			func(t *testing.T, w *Watcher, data coretypes.ResultEvent, key string) {
				checkAndSetInTransit(t, data, w, ibcTimeoutTxHash, "timeout_packet", key)
				HandleMessage(w, data)
			},
		},
		{
			"Handle swap transaction",
			swapTransactionEvent(t),
			logger,
			swapTxHash,
			"complete",
			func(t *testing.T, w *Watcher, data coretypes.ResultEvent, _ string) {
				HandleMessage(w, data)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			watcherInstance := &Watcher{
				l:     tt.logger,
				d:     dbInstance,
				store: s,
				Name:  defaultChainName,
			}
			require.NoError(t, s.CreateTicket(watcherInstance.Name, tt.txHash, testOwner))
			key := store.GetKey(defaultChainName, tt.txHash)
			tt.validateFn(t, watcherInstance, tt.event, key)
			if tt.expStatus != "" {
				ticket, err := s.Get(key)
				require.NoError(t, err)
				require.Equal(t, tt.expStatus, ticket.Status)
			}
		})
	}
}

func checkAndSetInTransit(t *testing.T, data coretypes.ResultEvent, w *Watcher, txHash, eventType, key string) {
	srcChannel, ok := data.Events[eventType+`.packet_src_channel`]
	require.True(t, ok)
	pktSeq, ok := data.Events[eventType+`.packet_sequence`]
	require.True(t, ok)
	require.GreaterOrEqual(t, 1, len(srcChannel))
	require.GreaterOrEqual(t, 1, len(pktSeq))
	require.NoError(t, s.SetInTransit(key, w.Name, srcChannel[0], pktSeq[0], txHash, w.Name, defaultHeight))
}

func checkDenomExists(t *testing.T, w *Watcher, denom string, expected bool) {
	// check pool denom is updated in db
	c, err := w.d.Chain(w.Name)
	require.NoError(t, err)
	found := false
	for _, dd := range c.Denoms {
		if dd.Name == denom {
			found = true
			break
		}
	}
	require.Equal(t, expected, found)
}

func TestHandleCosmosHubLPCreated(t *testing.T) {
	defer store.ResetTestStore(mr, s)

	watcherInstance := &Watcher{
		l:     logger,
		d:     dbInstance,
		store: s,
		Name:  defaultChainName,
	}

	re := createPoolEvent(t, true)
	require.NoError(t, s.CreateTicket(watcherInstance.Name, createPoolTxHash, testOwner))
	key := store.GetKey(defaultChainName, createPoolTxHash)

	tests := []struct {
		name        string
		chainName   string
		data        coretypes.ResultEvent
		denomStored bool
	}{
		{
			"Handle Created LP - wrong chainName",
			"test-chain",
			re,
			false,
		},
		{
			"Handle Created LP - empty data",
			defaultChainName,
			coretypes.ResultEvent{},
			false,
		},
		{
			"Handle Created LP - incomplete data",
			defaultChainName,
			coretypes.ResultEvent{Events: map[string][]string{
				"create_pool.pool_coin_denom": {"testpooltoken"},
			}},
			false,
		},
		{
			"Handle Created LP - valid data",
			defaultChainName,
			re,
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			HandleCosmosHubLPCreated(watcherInstance, tt.data, tt.chainName, key, defaultHeight)
			checkDenomExists(t, watcherInstance, newPoolDenom, tt.denomStored)
		})
	}
}

func TestHandleSwapTransaction(t *testing.T) {
	watcherInstance := &Watcher{
		l:     logger,
		d:     dbInstance,
		store: s,
		Name:  defaultChainName,
	}

	re := swapTransactionEvent(t)
	defaultKey := store.GetKey(defaultChainName, swapTxHash)

	tests := []struct {
		name      string
		data      coretypes.ResultEvent
		key       string
		expStatus string
	}{
		{
			"Handle swap transaction - empty data",
			coretypes.ResultEvent{},
			defaultKey,
			"pending",
		},
		{
			"Handle swap transaction - incomplete data",
			coretypes.ResultEvent{Events: map[string][]string{
				"swap_within_batch.pool_id": {"5"},
			}},
			defaultKey,
			"pending",
		},
		{
			"Handle swap transaction - incomplete data",
			coretypes.ResultEvent{Events: map[string][]string{
				"swap_within_batch.pool_id": {"5"},
			}},
			defaultKey,
			"pending",
		},
		{
			"Handle swap transaction - wrong offer fee",
			coretypes.ResultEvent{Events: map[string][]string{
				"swap_within_batch.pool_id":               {"5"},
				"swap_within_batch.offer_coin_fee_amount": {"testamount"},
				"swap_within_batch.offer_coin_denom":      {"testdenom"},
			}},
			defaultKey,
			"complete",
		},
		{
			"Handle swap transaction - wrong key",
			re,
			"testkey",
			"",
		},
		{
			"Handle swap transaction - valid data",
			re,
			defaultKey,
			"complete",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer store.ResetTestStore(mr, s)
			require.NoError(t, s.CreateTicket(watcherInstance.Name, swapTxHash, testOwner))
			HandleSwapTransaction(watcherInstance, tt.data, watcherInstance.Name, tt.key, defaultHeight)
			ticket, err := s.Get(tt.key)
			if tt.expStatus != "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
			require.Equal(t, tt.expStatus, ticket.Status)
		})
	}
}

func TestHandleIBCSenderEvent(t *testing.T) {
	watcherInstance := &Watcher{
		l:     logger,
		d:     dbInstance,
		store: s,
		Name:  defaultChainName,
	}

	re := ibcTransferEvent(t)
	defaultKey := store.GetKey(defaultChainName, ibcTransferTxHash)

	tests := []struct {
		name      string
		data      coretypes.ResultEvent
		expStatus string
	}{
		{
			"Handle ibc send transaction - empty data",
			coretypes.ResultEvent{},
			"pending",
		},
		{
			"Handle ibc send transaction - wrong source port",
			coretypes.ResultEvent{Events: map[string][]string{
				"send_packet.packet_src_port": {"send"},
			}},
			"pending",
		},
		{
			"Handle ibc send transaction - incomplete data",
			coretypes.ResultEvent{Events: map[string][]string{
				"send_packet.packet_src_port": {"transfer"},
			}},
			"pending",
		},
		{
			"Handle ibc send transaction - valid data",
			re,
			"transit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer store.ResetTestStore(mr, s)
			require.NoError(t, s.CreateTicket(watcherInstance.Name, ibcTransferTxHash, testOwner))
			HandleIBCSenderEvent(watcherInstance, tt.data, watcherInstance.Name, ibcTransferTxHash, defaultKey, defaultHeight)
			ticket, err := s.Get(defaultKey)
			require.NoError(t, err)
			require.Equal(t, tt.expStatus, ticket.Status)
		})
	}
}

func TestHandleIBCReceivePktEvent(t *testing.T) {
	watcherInstance := &Watcher{
		l:     logger,
		d:     dbInstance,
		store: s,
		Name:  defaultChainName,
	}

	eventWithSuccessAck := ibcReceivePacketEvent(t, true)
	eventWithFailedAck := ibcReceivePacketEvent(t, false)
	defaultKey := store.GetKey(defaultChainName, ibcReceiveTxHash)

	tests := []struct {
		name       string
		data       coretypes.ResultEvent
		useDefault bool
		expStatus  string
	}{
		{
			"Handle ibc receive packet - empty data",
			coretypes.ResultEvent{},
			true,
			"transit",
		},
		{
			"Handle ibc receive packet - wrong source port",
			coretypes.ResultEvent{Events: map[string][]string{
				"recv_packet.packet_src_port": {"send"},
			}},
			true,
			"transit",
		},
		{
			"Handle ibc receive packet - incomplete data",
			coretypes.ResultEvent{Events: map[string][]string{
				"recv_packet.packet_src_port": {"transfer"},
			}},
			true,
			"transit",
		},
		{
			"Handle ibc receive packet successful transaction",
			eventWithSuccessAck,
			false,
			"IBC_receive_success",
		},
		{
			"Handle ibc receive packet failed transaction",
			eventWithFailedAck,
			false,
			"IBC_receive_failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer store.ResetTestStore(mr, s)
			require.NoError(t, s.CreateTicket(watcherInstance.Name, ibcReceiveTxHash, testOwner))
			if tt.useDefault {
				require.NoError(t, s.SetInTransit(defaultKey, watcherInstance.Name, defaultChannel, defaultPktSeq,
					ibcAckTxHash, watcherInstance.Name, defaultHeight))
			} else {
				checkAndSetInTransit(t, tt.data, watcherInstance, ibcReceiveTxHash, "recv_packet", defaultKey)
			}
			HandleIBCReceivePacket(watcherInstance, tt.data, watcherInstance.Name, ibcReceiveTxHash, defaultHeight)
			ticket, err := s.Get(defaultKey)
			require.NoError(t, err)
			require.Equal(t, tt.expStatus, ticket.Status)
		})
	}
}

func TestHandleIBCAckPktEvent(t *testing.T) {
	watcherInstance := &Watcher{
		l:     logger,
		d:     dbInstance,
		store: s,
		Name:  defaultChainName,
	}

	// creating two receive tx events with different packet acknowledgements
	eventWithPktError := ibcAckTxEvent(t, true)
	eventWithoutPktError := ibcAckTxEvent(t, false)
	defaultKey := store.GetKey(defaultChainName, ibcAckTxHash)

	tests := []struct {
		name       string
		data       coretypes.ResultEvent
		useDefault bool
		expStatus  string
	}{
		{
			"Handle ibc ack packet - empty data",
			coretypes.ResultEvent{},
			true,
			"transit",
		},
		{
			"Handle ibc ack packet - incomplete data",
			coretypes.ResultEvent{Events: map[string][]string{
				"acknowledge_packet.packet_src_channel": {"channel-0"},
			}},
			true,
			"transit",
		},
		{
			"Handle ibc ack packet without token packet error",
			eventWithoutPktError,
			false,
			"transit",
		},
		{
			"Handle ibc ack packet with token packet error",
			eventWithPktError,
			false,
			"Tokens_unlocked_ack",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer store.ResetTestStore(mr, s)
			require.NoError(t, s.CreateTicket(watcherInstance.Name, ibcAckTxHash, testOwner))
			if tt.useDefault {
				require.NoError(t, s.SetInTransit(defaultKey, watcherInstance.Name, defaultChannel, defaultPktSeq,
					ibcAckTxHash, watcherInstance.Name, defaultHeight))
			} else {
				checkAndSetInTransit(t, tt.data, watcherInstance, ibcAckTxHash, "acknowledge_packet", defaultKey)
			}
			HandleIBCAckPacket(watcherInstance, tt.data, watcherInstance.Name, ibcAckTxHash, defaultHeight)
			ticket, err := s.Get(defaultKey)
			require.NoError(t, err)
			require.Equal(t, tt.expStatus, ticket.Status)
		})
	}
}

func TestHandleIBCTimeoutPktEvent(t *testing.T) {
	watcherInstance := &Watcher{
		l:     logger,
		d:     dbInstance,
		store: s,
		Name:  defaultChainName,
	}

	re := ibcTimeoutEvent(t)
	defaultKey := store.GetKey(defaultChainName, ibcTimeoutTxHash)

	tests := []struct {
		name       string
		data       coretypes.ResultEvent
		useDefault bool
		expStatus  string
	}{
		{
			"Handle ibc timeout packet - empty data",
			coretypes.ResultEvent{},
			true,
			"transit",
		},
		{
			"Handle ibc timeout packet - incomplete data",
			coretypes.ResultEvent{Events: map[string][]string{
				"timeout_packet.packet_src_channel": {"channel-0"},
			}},
			true,
			"transit",
		},
		{
			"Handle succesful ibc timeout packet transaction",
			re,
			false,
			"Tokens_unlocked_timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer store.ResetTestStore(mr, s)
			require.NoError(t, s.CreateTicket(watcherInstance.Name, ibcTimeoutTxHash, testOwner))
			if tt.useDefault {
				require.NoError(t, s.SetInTransit(defaultKey, watcherInstance.Name, defaultChannel, defaultPktSeq,
					ibcAckTxHash, watcherInstance.Name, defaultHeight))
			} else {
				checkAndSetInTransit(t, tt.data, watcherInstance, ibcTimeoutTxHash, "timeout_packet", defaultKey)
			}
			HandleIBCTimeoutPacket(watcherInstance, tt.data, watcherInstance.Name, ibcTimeoutTxHash, defaultHeight)
			ticket, err := s.Get(defaultKey)
			require.NoError(t, err)
			require.Equal(t, tt.expStatus, ticket.Status)
		})
	}
}
