package integration

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"testing"

	"github.com/allinbits/emeris-rpcwatcher/rpcwatcher/database"
	"github.com/cockroachdb/cockroach-go/v2/testserver"
	relayerCmd "github.com/cosmos/relayer/cmd"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
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
}

func (s *IntegrationTestSuite) SetupSuite() {
	s.tempDir = s.T().TempDir()
	s.chains = spinUpTestChains(s.T(), testchains...)
	s.Require().Len(s.chains, len(testchains))
	cmd := exec.Command("/bin/sh", "setup/relayer-setup.sh", s.tempDir, defaultRlyDir, defaultRlyPath,
		s.chains[0].chainID, s.chains[0].accountInfo.denom, s.chains[0].accountInfo.prefix, s.chains[0].accountInfo.seed, s.chains[0].rpcPort,
		s.chains[1].chainID, s.chains[1].accountInfo.denom, s.chains[1].accountInfo.prefix, s.chains[1].accountInfo.seed, s.chains[1].rpcPort,
	)
	require.NotNil(s.T(), cmd)
	err := cmd.Run()
	require.NoError(s.T(), err)
	data, err := ioutil.ReadFile(fmt.Sprintf("%s/%s/config/config.yaml", s.tempDir, defaultRlyDir))
	require.NoError(s.T(), err)
	var relayerCfg relayerCmd.Config
	require.NoError(s.T(), yaml.Unmarshal(data, &relayerCfg))
	require.NotNil(s.T(), relayerCfg.Paths[defaultRlyPath])
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
	require.NoError(s.T(), err)
	s.T().Log(chains)
}

func (s *IntegrationTestSuite) TestDummy() {
	// time.Sleep(10 * time.Minute)
	s.Require().True(true)
}

func (s *IntegrationTestSuite) TearDownSuite() {
	s.T().Log("tearing down integration test suite")
}

func TestIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}
