package rpcwatcher

import (
	"encoding/json"
	"log"
	"os"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/allinbits/emeris-rpcwatcher/rpcwatcher/database"
	dbutils "github.com/allinbits/emeris-rpcwatcher/utils/database"
	"github.com/allinbits/emeris-rpcwatcher/utils/logging"
	"github.com/allinbits/emeris-rpcwatcher/utils/store"
	"github.com/cockroachdb/cockroach-go/v2/testserver"
	"github.com/stretchr/testify/require"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
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
		eventType  string
		logger     *zap.SugaredLogger
		resultData string
		txHash     string
		expStatus  string
		validateFn func(*testing.T, *Watcher, coretypes.ResultEvent, string)
	}{
		{
			"Handle message without logger",
			nonIBCTransferEvent,
			nil,
			resultCodeZero,
			nonIBCTransferTxHash,
			"",
			func(t *testing.T, w *Watcher, data coretypes.ResultEvent, _ string) {
				require.Panics(t, func() { HandleMessage(w, data) })
			},
		},
		{
			"Handle non IBC transaction",
			nonIBCTransferEvent,
			logger,
			resultCodeZero,
			nonIBCTransferTxHash,
			"complete",
			func(t *testing.T, w *Watcher, data coretypes.ResultEvent, _ string) {
				HandleMessage(w, data)
			},
		},
		{
			"Handle failed non IBC transaction",
			nonIBCTransferEvent,
			logger,
			resultCodeNonZero,
			nonIBCTransferTxHash,
			"failed",
			func(t *testing.T, w *Watcher, data coretypes.ResultEvent, _ string) {
				HandleMessage(w, data)
			},
		},
		{
			"Handle create LP transaction",
			createPoolEvent,
			logger,
			resultCodeZero,
			createPoolTxHash,
			"complete",
			func(t *testing.T, w *Watcher, data coretypes.ResultEvent, _ string) {
				HandleMessage(w, data)
				// check pool denom is updated in db
				c, err := w.d.Chain(w.Name)
				require.NoError(t, err)
				found := false
				for _, dd := range c.Denoms {
					if dd.Name == defaultPoolDenom {
						found = true
						break
					}
				}
				require.True(t, found)
			},
		},
		{
			"Handle ibc transfer",
			ibcTransferEvent,
			logger,
			resultCodeZero,
			ibcTransferTxHash,
			"transit",
			func(t *testing.T, w *Watcher, data coretypes.ResultEvent, _ string) {
				HandleMessage(w, data)
			},
		},
		{
			"Handle IBC receive packet transaction",
			ibcReceivePacketEvent,
			logger,
			resultCodeZero,
			ibcReceiveTxHash,
			"IBC_receive_success",
			func(t *testing.T, w *Watcher, data coretypes.ResultEvent, key string) {
				require.NoError(t, s.SetInTransit(key, defaultChainName, defaultChannel, defaultReceivePktSeq,
					ibcReceiveTxHash, defaultChainName, defaultHeight))
				HandleMessage(w, data)
			},
		},
		{
			"Handle IBC acknowledge packet transaction",
			ibcAckTxEvent,
			logger,
			resultCodeZero,
			ibcAckTxHash,
			"Tokens_unlocked_ack",
			func(t *testing.T, w *Watcher, data coretypes.ResultEvent, key string) {
				require.NoError(t, s.SetInTransit(key, defaultChainName, defaultChannel, defaultAckPktSeq,
					ibcAckTxHash, defaultChainName, defaultHeight))
				HandleMessage(w, data)
			},
		},
		{
			"Handle IBC timeout packet transaction",
			ibcTimeoutEvent,
			logger,
			resultCodeZero,
			ibcTimeoutTxHash,
			"Tokens_unlocked_timeout",
			func(t *testing.T, w *Watcher, data coretypes.ResultEvent, key string) {
				require.NoError(t, s.SetInTransit(key, defaultChainName, defaultChannel, defaultTimeoutPktSeq,
					ibcTimeoutTxHash, defaultChainName, defaultHeight))
				HandleMessage(w, data)
			},
		},
		{
			"Handle swap transaction",
			swapTransactionEvent,
			logger,
			resultCodeZero,
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
			var re coretypes.ResultEvent
			require.NoError(t, json.Unmarshal([]byte(tt.eventType), &re))
			var d types.EventDataTx
			require.NoError(t, json.Unmarshal([]byte(tt.resultData), &d))
			re.Data = d
			require.NoError(t, s.CreateTicket(watcherInstance.Name, tt.txHash, testOwner))
			key := store.GetKey(defaultChainName, tt.txHash)
			tt.validateFn(t, watcherInstance, re, key)
			if tt.expStatus != "" {
				ticket, err := s.Get(key)
				require.NoError(t, err)
				require.Equal(t, tt.expStatus, ticket.Status)
			}
		})
	}
}
