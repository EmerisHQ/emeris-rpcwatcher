//go:build !race
// +build !race

// TODO: fix race conditions and avoid !race
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/avast/retry-go"
	"github.com/cockroachdb/cockroach-go/v2/testserver"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/rest"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	relayerCmd "github.com/cosmos/relayer/cmd"
	"github.com/emerishq/emeris-rpcwatcher/rpcwatcher"
	"github.com/emerishq/emeris-rpcwatcher/rpcwatcher/database"
	"github.com/emerishq/emeris-utils/logging"
	"github.com/emerishq/emeris-utils/store"
	liquiditytypes "github.com/gravity-devs/liquidity/x/liquidity/types"
	"github.com/ory/dockertest/v3"
	"github.com/stretchr/testify/suite"
	tmjson "github.com/tendermint/tendermint/libs/json"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	"google.golang.org/grpc"
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
	dbInstance *database.Instance
	mr         *miniredis.Miniredis
	store      *store.Store
	ts         testserver.TestServer
	network    *dockertest.Network
	relayer    *dockertest.Resource
}

func (s *IntegrationTestSuite) SetupSuite() {
	// uses a sensible default on windows (tcp/http) and linux/osx (socket)
	pool, err := dockertest.NewPool("")
	if err != nil {
		s.Require().NoError(fmt.Errorf("could not connect to docker at %s: %w", pool.Client.Endpoint(), err))
	}
	s.network, err = pool.CreateNetwork("test-on-start")
	s.Require().NoError(err)

	s.chains = spinUpTestChains(s.T(), pool, s.network, testchains...)
	s.Require().Len(s.chains, len(testchains))

	s.relayer = spinRelayer(s.T(), pool, s.network, defaultRlyPath, s.chains[0].chainID, s.chains[0].accountInfo.primaryDenom,
		s.chains[0].accountInfo.prefix, s.chains[0].accountInfo.seed, getRPCAddress(s.chains[0].nodeAddress, defaultRPCPort),
		s.chains[1].chainID, s.chains[1].accountInfo.primaryDenom, s.chains[1].accountInfo.prefix, s.chains[1].accountInfo.seed,
		getRPCAddress(s.chains[1].nodeAddress, defaultRPCPort),
	)

	data := s.executeDockerCmd(s.relayer, []string{"rly", "cfg", "show", "--json"})
	var relayerCfg relayerCmd.Config
	s.Require().NoError(json.Unmarshal(data.Bytes(), &relayerCfg))
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
		watcher, err := rpcwatcher.NewWatcher(getRPCAddress(chain.nodeAddress, defaultRPCPort), chain.chainID, logger, "",
			getGRPCAddress(chain.nodeAddress, defaultGRPCPort), s.dbInstance, s.store, rpcwatcher.EventsToSubTo, eventMappings)
		s.Require().NoError(err)

		err = s.store.SetWithExpiry(chain.chainID, "true", 0)
		s.Require().NoError(err)

		rpcwatcher.Start(watcher, context.Background())
	}
}

func (s *IntegrationTestSuite) TestNonIBCTransfer() {
	chain := s.chains[0]
	stdOut := s.executeDockerCmd(
		chain.resource,
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
		chain1.resource,
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
	time.Sleep(100 * time.Second)

	// test ibc recv packet
	stdOut = s.executeDockerCmd(
		chain2.resource,
		[]string{chain2.binaryName, "q", "txs", "--events", fmt.Sprintf("'message.action=recv_packet&fungible_token_packet.amount=%s'", amount)},
	)
	res := s.UnmarshalSearchTxs(stdOut.Bytes())
	packetSeq := getEventValueFromTx(*res.Txs[0], "recv_packet", "packet_sequence")
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
		chain1.resource,
		[]string{chain1.binaryName, "q", "txs", "--events", fmt.Sprintf("'message.action=acknowledge_packet&acknowledge_packet.packet_sequence=%s'", packetSeq)},
	)
	ackRes := s.UnmarshalSearchTxs(stdOut.Bytes())
	// check acknowledgement error found
	ackErr := getEventValueFromTx(*ackRes.Txs[0], "fungible_token_packet", "error")
	if ackErr != "" {
		ticket, err = s.store.Get(key)
		s.Require().NoError(err)
		s.Require().True(checkTxHashEntry(ticket, store.TxHashEntry{
			Chain:  chain1.chainID,
			Status: "Tokens_unlocked_ack",
			TxHash: res.Txs[0].TxHash,
		}))
	}
}

func (s *IntegrationTestSuite) TestIBCTimeoutTransfer() {
	s.Require().GreaterOrEqual(len(s.chains), 2)
	chain1 := s.chains[0]
	chain2 := s.chains[1]

	stdOut := s.executeDockerCmd(
		s.relayer,
		[]string{"rly", "q", "balance", chain1.chainID},
	)

	initialBalance := stdOut.String()

	amount := "120"

	// test ibc transfer with less packet timeout
	stdOut = s.executeDockerCmd(
		chain1.resource,
		[]string{chain1.binaryName, "tx", "ibc-transfer", "transfer", defaultPort, chain1.channels[chain2.chainID],
			chain2.accountInfo.address, fmt.Sprintf("%s%s", amount, chain1.accountInfo.primaryDenom),
			fmt.Sprintf("--from=%s", chain1.accountInfo.keyname), "--keyring-backend=test", "-y",
			fmt.Sprintf("--chain-id=%s", chain1.chainID), "--packet-timeout-timestamp=1",
			"--broadcast-mode=async",
		},
	)
	txRes := s.UnmarshalTx(stdOut.Bytes())
	txHash := txRes.TxHash
	err := s.store.CreateTicket(chain1.chainID, txHash, chain1.accountInfo.address)
	s.Require().NoError(err)

	// Wait for relayer to relay tx
	time.Sleep(30 * time.Second)

	stdOut = bytes.Buffer{}
	err = retry.Do(func() error {
		var stdErr bytes.Buffer
		exitCode, err := s.relayer.Exec(
			[]string{"rly", "tx", "relay-packets", defaultRlyPath},
			dockertest.ExecOptions{
				StdOut: &stdOut,
				StdErr: &stdErr,
			})
		if err != nil || exitCode != 0 {
			return fmt.Errorf("got error executing relay-packets commands: %s", stdErr.String())
		}
		return nil
	})
	s.Require().NoError(err, stdOut.String())

	// Wait for rpcwatcher to catch timeout tx
	time.Sleep(60 * time.Second)

	// test ibc recv packet
	stdOut = s.executeDockerCmd(
		chain1.resource,
		[]string{chain1.binaryName, "q", "txs", "--events", "'message.action=timeout_packet'"},
	)
	res := s.UnmarshalSearchTxs(stdOut.Bytes())
	key := store.GetKey(chain1.chainID, txHash)
	ticket, err := s.store.Get(key)
	s.Require().NoError(err)
	s.Require().True(checkTxHashEntry(ticket, store.TxHashEntry{
		Chain:  chain1.chainID,
		Status: "Tokens_unlocked_timeout",
		TxHash: res.Txs[0].TxHash,
	}))

	// check whether refund is received as tx was timeout transfer
	stdOut = s.executeDockerCmd(
		s.relayer,
		[]string{"rly", "q", "balance", chain1.chainID},
	)
	finalBalance := stdOut.String()
	s.Require().Equal(initialBalance, finalBalance)
}

func (s *IntegrationTestSuite) TestLiquidityPoolTxs() {
	chain := s.chains[0]
	s.Require().GreaterOrEqual(len(chain.denoms), 2)

	poolAmount := 1000000
	// test create-pool
	stdOut := s.executeDockerCmd(
		chain.resource,
		[]string{chain.binaryName, "tx", "liquidity", "create-pool", "1",
			fmt.Sprintf("%d%s,%d%s", poolAmount, chain.accountInfo.primaryDenom, poolAmount, chain.denoms[1].denom),
			fmt.Sprintf("--from=%s", chain.accountInfo.keyname), "--keyring-backend=test", "-y",
			fmt.Sprintf("--chain-id=%s", chain.chainID),
		},
	)
	txRes := s.UnmarshalTx(stdOut.Bytes())
	txHash := txRes.TxHash
	err := s.store.CreateTicket(chain.chainID, txHash, chain.accountInfo.address)
	s.Require().NoError(err)

	time.Sleep(30 * time.Second)

	out := s.executeDockerCmd(chain.resource, []string{chain.binaryName, "q", "tx", txHash})
	txRes = s.UnmarshalTx(out.Bytes())
	poolDenom := getEventValueFromTx(txRes, "create_pool", "pool_coin_denom")
	s.checkDenomExists(chain.chainID, poolDenom, true)
	ticket, err := s.store.Get(store.GetKey(chain.chainID, txHash))
	s.Require().NoError(err)
	s.Require().Equal("complete", ticket.Status)

	// check cache supply updated with new pool denom
	bz, err := s.store.GetSupply()
	s.Require().NoError(err)
	var supply banktypes.QueryTotalSupplyResponse
	err = tmjson.Unmarshal(bz, &supply)
	s.Require().NoError(err)

	expCoin := sdk.NewCoin(poolDenom, sdk.NewInt(int64(poolAmount)))
	found := false
	for _, coin := range supply.Supply {
		if coin.Equal(expCoin) {
			found = true
		}
	}
	s.Require().True(found, "cache supply not updated with new pool denom")

	// check cache pools updated with new pool
	bz, err = s.store.GetPools()
	s.Require().NoError(err)

	var pools liquiditytypes.QueryLiquidityPoolsResponse
	err = tmjson.Unmarshal(bz, &pools)
	s.Require().NoError(err)

	found = false
	for _, pool := range pools.Pools {
		if pool.PoolCoinDenom == poolDenom {
			found = true
		}
	}
	s.Require().True(found, "cache pools not updated with new pool")

	// test swap transaction
	poolID := getEventValueFromTx(txRes, "create_pool", "pool_id")
	stdOut = s.executeDockerCmd(
		chain.resource,
		[]string{chain.binaryName, "tx", "liquidity", "swap", poolID, "1",
			fmt.Sprintf("10000%s", chain.denoms[1].denom), chain.accountInfo.primaryDenom, "0.019", "0.003",
			fmt.Sprintf("--from=%s", chain.accountInfo.keyname), "--keyring-backend=test", "-y",
			fmt.Sprintf("--chain-id=%s", chain.chainID), "--broadcast-mode=async",
		},
	)
	txRes = s.UnmarshalTx(stdOut.Bytes())
	txHash = txRes.TxHash
	err = s.store.CreateTicket(chain.chainID, txHash, chain.accountInfo.address)
	s.Require().NoError(err)

	time.Sleep(15 * time.Second)
	out = s.executeDockerCmd(chain.resource, []string{chain.binaryName, "q", "tx", txHash})
	txRes = s.UnmarshalTx(out.Bytes())

	// check stored swap fees
	offerCoinFee := getEventValueFromTx(txRes, "swap_within_batch", "offer_coin_fee_amount")
	offerCoinDenom := getEventValueFromTx(txRes, "swap_within_batch", "offer_coin_denom")
	offerCoinAmountInt, ok := sdk.NewIntFromString(offerCoinFee)
	s.Require().True(ok)
	coins := sdk.NewCoins(sdk.NewCoin(offerCoinDenom, offerCoinAmountInt))
	fees, err := s.store.GetSwapFees(poolID)
	s.Require().NoError(err)
	s.Require().Equal(coins.String(), fees.String())

	ticket, err = s.store.Get(store.GetKey(chain.chainID, txHash))
	s.Require().NoError(err)
	s.Require().Equal("complete", ticket.Status)
}

func (s *IntegrationTestSuite) TestHandleCosmosHubBlock() {
	chain := s.chains[0]
	s.Require().GreaterOrEqual(len(chain.denoms), 3)

	time.Sleep(15 * time.Second)

	stdOut := s.executeDockerCmd(
		chain.resource,
		[]string{chain.binaryName, "q", "block"},
	)

	var block coretypes.ResultBlock
	err := tmjson.Unmarshal(stdOut.Bytes(), &block)
	s.Require().NoError(err)
	height := block.Block.Height

	expected, err := rest.GetRequest(fmt.Sprintf("%s/block_results?height=%d", getRPCAddress(chain.nodeAddress, defaultRPCPort), height))
	s.Require().NoError(err)

	// wait for rpc watcher to store block data
	time.Sleep(10 * time.Second)

	bs := store.NewBlocks(s.store)
	storedBytes, err := bs.Block(height)
	s.Require().NoError(err, height)
	s.Require().Equal(string(expected), string(storedBytes))

	// create new pool
	stdOut = s.executeDockerCmd(
		chain.resource,
		[]string{chain.binaryName, "tx", "liquidity", "create-pool", "1",
			fmt.Sprintf("1000000%s,1000000%s", chain.accountInfo.primaryDenom, chain.denoms[2].denom),
			fmt.Sprintf("--from=%s", chain.accountInfo.keyname), "--keyring-backend=test", "-y",
			fmt.Sprintf("--chain-id=%s", chain.chainID),
		},
	)
	txRes := s.UnmarshalTx(stdOut.Bytes())
	txHash := txRes.TxHash

	time.Sleep(15 * time.Second)

	// check whether tx executed successfully
	out := s.executeDockerCmd(chain.resource, []string{chain.binaryName, "q", "tx", txHash})
	_ = s.UnmarshalTx(out.Bytes())

	grpcConn, err := grpc.Dial(
		getGRPCAddress(chain.nodeAddress, defaultGRPCPort),
		grpc.WithInsecure(),
	)
	s.Require().NoError(err)
	defer grpcConn.Close()

	liquidityQuery := liquiditytypes.NewQueryClient(grpcConn)

	// check pools are stored in cache
	poolsRes, err := liquidityQuery.LiquidityPools(context.Background(), &liquiditytypes.QueryLiquidityPoolsRequest{})
	s.Require().NoError(err)
	expected, err = s.store.Cdc.MarshalJSON(poolsRes)
	s.Require().NoError(err)

	time.Sleep(5 * time.Second)

	cachePools, err := s.store.GetPools()
	s.Require().NoError(err)
	s.Require().Equal(string(expected), string(cachePools))

	// check liquidity params are stored in cache
	paramsRes, err := liquidityQuery.Params(context.Background(), &liquiditytypes.QueryParamsRequest{})
	s.Require().NoError(err)
	expected, err = s.store.Cdc.MarshalJSON(paramsRes)
	s.Require().NoError(err)

	time.Sleep(5 * time.Second)

	cacheParams, err := s.store.GetParams()
	s.Require().NoError(err)
	s.Require().Equal(string(expected), string(cacheParams))
}

func (s *IntegrationTestSuite) TearDownSuite() {
	s.T().Log("tearing down integration test suite")
	s.ts.Stop()
	s.network.Close()
}

func TestIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}

func (s *IntegrationTestSuite) executeDockerCmd(resource *dockertest.Resource, command []string) bytes.Buffer {
	var stdOut, stdErr bytes.Buffer
	exitCode, err := resource.Exec(
		command,
		dockertest.ExecOptions{
			StdOut: &stdOut,
			StdErr: &stdErr,
		})
	s.Require().NoError(err)
	s.Require().Equal(0, exitCode, stdErr.String())
	return stdOut
}

func (s *IntegrationTestSuite) UnmarshalTx(out []byte) sdk.TxResponse {
	var res sdk.TxResponse
	err := tmjson.Unmarshal(out, &res)
	s.Require().NoError(err)
	s.Require().Equal(uint32(0), res.Code, string(out))
	return res
}

func (s *IntegrationTestSuite) UnmarshalSearchTxs(out []byte) sdk.SearchTxsResult {
	var res sdk.SearchTxsResult
	err := tmjson.Unmarshal(out, &res)
	s.Require().NoError(err)
	s.Require().Len(res.Txs, 1)
	return res
}

func (s *IntegrationTestSuite) checkDenomExists(chainName, denom string, expected bool) {
	// check pool denom is updated in db
	c, err := s.dbInstance.Chain(chainName)
	s.Require().NoError(err)
	found := false
	for _, dd := range c.Denoms {
		if dd.Name == denom {
			found = true
			break
		}
	}
	s.Require().Equal(expected, found)
}
