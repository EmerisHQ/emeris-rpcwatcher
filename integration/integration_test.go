package integration

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

var testchains = []testChain{gaiaTestChain, akashTestChain}

type IntegrationTestSuite struct {
	suite.Suite

	chains  []testChain
	tempDir string
}

func (s *IntegrationTestSuite) SetupSuite() {
	s.tempDir = s.T().TempDir()
	s.chains = spinUpTestChains(s.T(), testchains...)
	s.Require().Len(s.chains, len(testchains))
	cmd := exec.Command("/bin/sh", "setup/relayer-setup.sh", s.tempDir,
		s.chains[0].chainID, s.chains[0].accountInfo.denom, s.chains[0].accountInfo.prefix, s.chains[0].accountInfo.seed, s.chains[0].rpcPort,
		s.chains[1].chainID, s.chains[1].accountInfo.denom, s.chains[1].accountInfo.prefix, s.chains[1].accountInfo.seed, s.chains[1].rpcPort,
	)
	require.NotNil(s.T(), cmd)
	err := cmd.Run()
	require.NoError(s.T(), err)
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
