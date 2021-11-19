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
)

// global variables for tests
var (
	s               *store.Store
	dbInstance      *database.Instance
	mr              *miniredis.Miniredis
	watcherInstance *Watcher
)

// TODO: use mocks for data in db
func TestMain(m *testing.M) {
	// setup test DB
	var ts testserver.TestServer
	ts, dbInstance = setupDB()
	defer ts.Stop()

	// logger
	l := logging.New(logging.LoggingConfig{
		LogPath: "",
		Debug:   true,
	})

	// setup test store
	mr, s = store.SetupTestStore()
	watcherInstance = &Watcher{
		l:     l,
		d:     dbInstance,
		store: s,
		Name:  "cosmos-hub",
	}
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
	var c coretypes.ResultEvent
	err := json.Unmarshal([]byte(nonIBCTransferEvent), &c)
	require.NoError(t, err)
	var d types.EventDataTx
	err = json.Unmarshal([]byte(resultCodeZero), &d)
	c.Data = d
	t.Log(c.Data)
	require.NoError(t, err)
	s.CreateTicket("cosmos-hub", "D88B758F52DD059958B55B5874105BDF421F0685BA7EB1ACBC7D04337D6466E0", "cosmos1vaa40n5naka7mav3za6kx40jckx6aa4nqvvx8a")
	HandleMessage(watcherInstance, c)
	ticket, err := s.Get(store.GetKey("cosmos-hub", "D88B758F52DD059958B55B5874105BDF421F0685BA7EB1ACBC7D04337D6466E0"))
	require.NoError(t, err)
	require.Equal(t, "complete", ticket.Status)
}
