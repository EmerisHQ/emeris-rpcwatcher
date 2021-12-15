package integration_test

import (
	"fmt"
	"testing"

	"github.com/allinbits/emeris-rpcwatcher/network"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	clitestutil "github.com/cosmos/cosmos-sdk/testutil/cli"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktestutil "github.com/cosmos/cosmos-sdk/x/bank/client/testutil"
	stakingcli "github.com/cosmos/cosmos-sdk/x/staking/client/cli"
	"github.com/stretchr/testify/suite"
)

type IntegrationTestSuite struct {
	suite.Suite

	cfg       network.Config
	network1  *network.Network
	network2  *network.Network
	chain1Val *network.Validator
	chain2Val *network.Validator
}

func (s *IntegrationTestSuite) SetupSuite() {
	s.T().Log("setting up integration test suite")

	s.cfg = network.DefaultConfig()

	s.network1 = network.New(s.T(), s.cfg)
	s.network2 = network.New(s.T(), s.cfg)

	err := s.network1.WaitForNextBlock()
	s.Require().NoError(err)

	err = s.network2.WaitForNextBlock()
	s.Require().NoError(err)

	s.chain1Val = s.network1.Validators[0]
	s.chain2Val = s.network2.Validators[0]

	s.T().Log("Val Addr...", s.chain1Val.Address.String(), s.chain1Val.ValAddress, s.chain2Val.Address.String(), s.chain2Val.ValAddress)

	info1, _, err := s.chain1Val.ClientCtx.Keyring.NewMnemonic("relayerAddr1", keyring.English, sdk.FullFundraiserPath, hd.Secp256k1)
	s.Require().NoError(err)

	chain1RelayAddr := sdk.AccAddress(info1.GetPubKey().Address())

	info2, _, err := s.chain2Val.ClientCtx.Keyring.NewMnemonic("relayerAddr2", keyring.English, sdk.FullFundraiserPath, hd.Secp256k1)
	s.Require().NoError(err)

	chain2RelayAddr := sdk.AccAddress(info2.GetPubKey().Address())

	err = s.network1.WaitForNextBlock()
	s.Require().NoError(err)

	err = s.network2.WaitForNextBlock()
	s.Require().NoError(err)

	s.T().Log("Addr...", chain1RelayAddr.String(), chain2RelayAddr.String())

	cmd := stakingcli.GetCmdQueryValidators()

	out, err := clitestutil.ExecTestCLICmd(s.chain1Val.ClientCtx, cmd, []string{"--output=json"})
	s.Require().NoError(err)

	s.T().Log("Out 1...", out)

	out, err = clitestutil.ExecTestCLICmd(s.chain2Val.ClientCtx, cmd, []string{"--output=json"})
	s.Require().NoError(err)

	s.T().Log("Out 2...", out)

	_, err = banktestutil.MsgSendExec(
		s.chain1Val.ClientCtx,
		s.chain1Val.Address,
		chain1RelayAddr,
		sdk.NewCoins(sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(10000000))),
		fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
		fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastBlock),
		fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(10))).String()),
		fmt.Sprintf("--node=%s", s.chain1Val.RPCAddress),
	)
	s.Require().NoError(err)

	_, err = banktestutil.MsgSendExec(
		s.chain2Val.ClientCtx,
		s.chain2Val.Address,
		chain2RelayAddr,
		sdk.NewCoins(sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(10000000))),
		fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
		fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastBlock),
		fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(10))).String()),
		fmt.Sprintf("--node=%s", s.chain2Val.RPCAddress),
	)
	s.Require().NoError(err)
}

func (s *IntegrationTestSuite) TearDownSuite() {
	s.T().Log("tearing down integration test suite")
	s.network1.Cleanup()
	s.network2.Cleanup()
}

func (s *IntegrationTestSuite) TestClientConnection() {
	s.Require().True(false)
}

func TestIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}
