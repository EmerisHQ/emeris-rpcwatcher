package integration

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

var testchains = []testChain{gaiaTestChain, akashTestChain}

type IntegrationTestSuite struct {
	suite.Suite

	chains []testChain
}

func (s *IntegrationTestSuite) SetupSuite() {
	// s.chains = spinUpTestChains(s.T(), testchains...)
	// s.Require().Len(s.chains, len(testchains))
	spinRelayer(s.T())
}

func (s *IntegrationTestSuite) TestDummy() {
	// time.Sleep(10 * time.Minute)
	s.Require().False(true)
}

func (s *IntegrationTestSuite) TearDownSuite() {
	s.T().Log("tearing down integration test suite")
}

func TestIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}
