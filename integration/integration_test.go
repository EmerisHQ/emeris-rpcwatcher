package integration

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os/exec"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/allinbits/emeris-rpcwatcher/rpcwatcher"
	"github.com/allinbits/emeris-rpcwatcher/rpcwatcher/database"
	"github.com/allinbits/emeris-utils/logging"
	"github.com/allinbits/emeris-utils/store"
	"github.com/cockroachdb/cockroach-go/v2/testserver"
	sdk "github.com/cosmos/cosmos-sdk/types"
	relayerCmd "github.com/cosmos/relayer/cmd"
	"github.com/ory/dockertest/v3"
	"github.com/stretchr/testify/suite"
	tmjson "github.com/tendermint/tendermint/libs/json"
	"gopkg.in/yaml.v2"
)

const (
	defaultRlyDir  = ".relayer_test"
	defaultRlyPath = "rly_test"
)

var testchains = []testChain{gaiaTestChain, akashTestChain}

type IntegrationTestSuite struct {
	suite.Suite

	chains     []testChain
	tempDir    string
	dbInstance *database.Instance
	mr         *miniredis.Miniredis
	store      *store.Store
}

func (s *IntegrationTestSuite) SetupSuite() {
	s.tempDir = s.T().TempDir()
	s.chains = spinUpTestChains(s.T(), testchains...)
	s.Require().Len(s.chains, len(testchains))
	cmd := exec.Command("/bin/sh", "setup/relayer-setup.sh", s.tempDir, defaultRlyDir, defaultRlyPath,
		s.chains[0].chainID, s.chains[0].accountInfo.denom, s.chains[0].accountInfo.prefix, s.chains[0].accountInfo.seed, s.chains[0].rpcPort,
		s.chains[1].chainID, s.chains[1].accountInfo.denom, s.chains[1].accountInfo.prefix, s.chains[1].accountInfo.seed, s.chains[1].rpcPort,
	)
	s.Require().NotNil(cmd)
	err := cmd.Run()
	s.Require().NoError(err)
	data, err := ioutil.ReadFile(fmt.Sprintf("%s/%s/config/config.yaml", s.tempDir, defaultRlyDir))
	s.Require().NoError(err)
	var relayerCfg relayerCmd.Config
	s.Require().NoError(yaml.Unmarshal(data, &relayerCfg))
	s.Require().NotNil(relayerCfg.Paths[defaultRlyPath])
	s.chains[0].channels = map[string]string{
		s.chains[1].chainID: relayerCfg.Paths[defaultRlyPath].Src.ChannelID,
	}
	s.chains[1].channels = map[string]string{
		s.chains[0].chainID: relayerCfg.Paths[defaultRlyPath].Dst.ChannelID,
	}
	// setup test DB
	var ts testserver.TestServer
	ts, s.dbInstance = database.SetupTestDB(getMigrations(s.chains))
	defer ts.Stop()

	chains, err := s.dbInstance.Chains()
	s.Require().NoError(err)
	s.Require().Len(chains, len(s.chains))

	// logger
	logger := logging.New(logging.LoggingConfig{
		LogPath: "",
		Debug:   true,
	})

	// setup test store
	s.mr, s.store = store.SetupTestStore()

	for _, chain := range s.chains {
		eventMappings := rpcwatcher.StandardMappings

		if chain.chainID == "cosmos-hub" { // special case, needs to observe new blocks too
			eventMappings = rpcwatcher.CosmosHubMappings
		}
		watcher, err := rpcwatcher.NewWatcher(fmt.Sprintf("http://localhost:%s", chain.rpcPort), chain.chainID, logger, "", s.dbInstance,
			s.store, rpcwatcher.EventsToSubTo, eventMappings)
		s.Require().NoError(err)

		err = s.store.SetWithExpiry(chain.chainID, "true", 0)
		s.Require().NoError(err)

		rpcwatcher.Start(watcher, context.Background())
	}
}

func (s *IntegrationTestSuite) TestNonIBCTransfer() {
	chain := s.chains[0]
	var stdOut bytes.Buffer
	var stdErr bytes.Buffer
	exitCode, err := s.chains[0].resource.Exec(
		[]string{chain.binaryName, "tx", "bank", "send", chain.accountInfo.address, chain.testAddress,
			fmt.Sprintf("100%s", chain.accountInfo.denom), fmt.Sprintf("--from=%s", chain.accountInfo.keyname),
			"--keyring-backend=test", "-y", fmt.Sprintf("--chain-id=%s", chain.chainID),
		},
		dockertest.ExecOptions{
			StdOut: &stdOut,
			StdErr: &stdErr,
		})
	s.Require().NoError(err)
	s.T().Log(stdOut.String())
	s.Require().Equal(0, exitCode, stdErr.String())
	var txRes sdk.TxResponse
	err = tmjson.Unmarshal(stdOut.Bytes(), &txRes)
	s.Require().NoError(err)
	s.Require().Equal(uint32(0), txRes.Code)
	txHash := txRes.TxHash
	err = s.store.CreateTicket(chain.chainID, txHash, chain.accountInfo.address)
	s.Require().NoError(err)
	// Wait for rpcwatcher to catch tx
	time.Sleep(5 * time.Second)
	ticket, err := s.store.Get(store.GetKey(chain.chainID, txHash))
	s.Require().NoError(err)
	s.Require().Equal("complete", ticket.Status)
	// s.Require().True(false)
}

// func (s *IntegrationTestSuite) TestDummy() {
// 	// time.Sleep(1 * time.Minute)
// 	s.Require().True(false)
// }

func (s *IntegrationTestSuite) TearDownSuite() {
	s.T().Log("tearing down integration test suite")
}

func TestIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}
