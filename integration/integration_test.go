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
	defaultPort    = "transfer"
)

var testchains = []testChain{gaiaTestChain, akashTestChain}

type IntegrationTestSuite struct {
	suite.Suite

	chains     []testChain
	tempDir    string
	dbInstance *database.Instance
	mr         *miniredis.Miniredis
	store      *store.Store
	ts         testserver.TestServer
}

func (s *IntegrationTestSuite) SetupSuite() {
	s.tempDir = s.T().TempDir()
	s.chains = spinUpTestChains(s.T(), testchains...)
	s.Require().Len(s.chains, len(testchains))
	cmd := exec.Command("/bin/sh", "setup/relayer-setup.sh", s.tempDir, defaultRlyDir, defaultRlyPath,
		s.chains[0].chainID, s.chains[0].accountInfo.primaryDenom, s.chains[0].accountInfo.prefix, s.chains[0].accountInfo.seed, s.chains[0].rpcPort,
		s.chains[1].chainID, s.chains[1].accountInfo.primaryDenom, s.chains[1].accountInfo.prefix, s.chains[1].accountInfo.seed, s.chains[1].rpcPort,
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
	s.ts, s.dbInstance = database.SetupTestDB(getMigrations(s.chains))

	chains, err := s.dbInstance.Chains()
	s.Require().NoError(err)
	s.Require().Len(chains, len(s.chains))
	s.T().Logf("Chains: %+v", chains)

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
	stdOut := s.executeDockerCmd(
		chain,
		[]string{chain.binaryName, "tx", "bank", "send", chain.accountInfo.address, chain.testAddress,
			fmt.Sprintf("100%s", chain.accountInfo.primaryDenom), fmt.Sprintf("--from=%s", chain.accountInfo.keyname),
			"--keyring-backend=test", "-y", fmt.Sprintf("--chain-id=%s", chain.chainID),
			"--broadcast-mode=async",
		},
	)
	txRes := s.UnmarshalTx(stdOut.Bytes())
	txHash := txRes.TxHash
	err := s.store.CreateTicket(chain.chainID, txHash, chain.accountInfo.address)
	s.Require().NoError(err)
	s.T().Log(stdOut.String())
	// Wait for rpcwatcher to catch tx
	time.Sleep(5 * time.Second)
	ticket, err := s.store.Get(store.GetKey(chain.chainID, txHash))
	s.Require().NoError(err)
	s.Require().Equal("complete", ticket.Status)
}

func (s *IntegrationTestSuite) TestIBCTransfer() {
	s.Require().GreaterOrEqual(len(s.chains), 2)
	chain1 := s.chains[0]
	chain2 := s.chains[1]

	amount := "110"

	// test ibc transfer
	stdOut := s.executeDockerCmd(
		chain1,
		[]string{chain1.binaryName, "tx", "ibc-transfer", "transfer", defaultPort, chain1.channels[chain2.chainID],
			chain2.accountInfo.address, fmt.Sprintf("%s%s", amount, chain1.accountInfo.primaryDenom),
			fmt.Sprintf("--from=%s", chain1.accountInfo.keyname), "--keyring-backend=test", "-y",
			fmt.Sprintf("--chain-id=%s", chain1.chainID),
			"--broadcast-mode=async",
		},
	)
	txRes := s.UnmarshalTx(stdOut.Bytes())
	txHash := txRes.TxHash
	err := s.store.CreateTicket(chain1.chainID, txHash, chain1.accountInfo.address)
	s.Require().NoError(err)

	// Wait for rpcwatcher to catch tx
	time.Sleep(5 * time.Second)
	key := store.GetKey(chain1.chainID, txHash)
	ticket, err := s.store.Get(key)
	s.Require().NoError(err)
	s.Require().Equal("transit", ticket.Status)

	// Wait for relayer to relay tx
	time.Sleep(60 * time.Second)

	ticket, err = s.store.Get(key)
	s.Require().NoError(err)
	s.T().Log("Ticket...", ticket)
	// test ibc recv packet
	stdOut = s.executeDockerCmd(
		chain2,
		[]string{chain2.binaryName, "q", "txs", "--events", fmt.Sprintf("'message.action=recv_packet&fungible_token_packet.amount=%s'", amount)},
	)
	res := s.UnmarshalSearchTxs(stdOut.Bytes())
	packetSeq := getPacketSequence(*res.Txs[0])
	s.Require().NotEmpty(packetSeq)
	ticket, err = s.store.Get(key)
	s.Require().NoError(err)
	s.Require().True(checkTxHashEntry(ticket, store.TxHashEntry{
		Chain:  chain2.chainID,
		Status: "IBC_receive_success",
		TxHash: res.Txs[0].TxHash,
	}))

	// test ibc ack packet
	stdOut = s.executeDockerCmd(
		chain1,
		[]string{chain1.binaryName, "q", "txs", "--events", fmt.Sprintf("'message.action=acknowledge_packet&acknowledge_packet.packet_sequence=%s'", packetSeq)},
	)
	ackRes := s.UnmarshalSearchTxs(stdOut.Bytes())
	// check acknowledgement error found
	ackErr := checkFungibleTokenErr(*ackRes.Txs[0])
	if ackErr {
		ticket, err = s.store.Get(key)
		s.Require().NoError(err)
		s.Require().True(checkTxHashEntry(ticket, store.TxHashEntry{
			Chain:  chain1.chainID,
			Status: "Tokens_unlocked_ack",
			TxHash: res.Txs[0].TxHash,
		}))
	}
}

func (s *IntegrationTestSuite) TestLiquidityPool() {
	chain1 := s.chains[0]
	// chain2 := s.chains[1]

	// test create-pool
	stdOut := s.executeDockerCmd(
		chain1,
		[]string{chain1.binaryName, "tx", "liquidity", "create-pool", "1",
			fmt.Sprintf("1000000%s,1000000%s", chain1.accountInfo.primaryDenom, "samoleans"),
			fmt.Sprintf("--from=%s", chain1.accountInfo.keyname), "--keyring-backend=test", "-y",
			fmt.Sprintf("--chain-id=%s", chain1.chainID),
		},
	)

	time.Sleep(30 * time.Second)

	txRes := s.UnmarshalTx(stdOut.Bytes())

	out := s.executeDockerCmd(chain1, []string{chain1.binaryName, "q", "tx", txRes.TxHash})
	s.T().Log("Out...", out.String())

	// s.Require().True(false)
}

func (s *IntegrationTestSuite) TearDownSuite() {
	s.T().Log("tearing down integration test suite")
	s.ts.Stop()
}

func TestIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}

func (s *IntegrationTestSuite) executeDockerCmd(chain testChain, command []string) bytes.Buffer {
	var stdOut, stdErr bytes.Buffer
	exitCode, err := chain.resource.Exec(
		command,
		dockertest.ExecOptions{
			StdOut: &stdOut,
			StdErr: &stdErr,
		})
	s.Require().NoError(err)
	s.T().Log(stdOut.String())
	s.Require().Equal(0, exitCode, stdErr.String())
	return stdOut
}

func (s *IntegrationTestSuite) UnmarshalTx(out []byte) sdk.TxResponse {
	s.T().Log("Input:", string(out))
	var res sdk.TxResponse
	err := tmjson.Unmarshal(out, &res)
	s.Require().NoError(err)
	s.T().Logf("Res: %+v", res)
	s.T().Log("Code...", res.Code)
	s.Require().Equal(uint32(0), res.Code)
	return res
}

func (s *IntegrationTestSuite) UnmarshalSearchTxs(out []byte) sdk.SearchTxsResult {
	var res sdk.SearchTxsResult
	err := tmjson.Unmarshal(out, &res)
	s.Require().NoError(err)
	s.Require().Len(res.Txs, 1)
	return res
}
