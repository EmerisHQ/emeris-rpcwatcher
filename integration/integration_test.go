package integration

import (
	"fmt"
	"testing"

	"github.com/ory/dockertest/v3"
	"github.com/stretchr/testify/suite"
)

var testchains = []testChain{gaiaTestChain, akashTestChain}

type IntegrationTestSuite struct {
	suite.Suite

	chains []testChain
}

func (s *IntegrationTestSuite) SetupSuite() {
	pool, err := dockertest.NewPool("")
	if err != nil {
		s.Require().NoError(fmt.Errorf("could not connect to docker at %s: %w", pool.Client.Endpoint(), err))
	}
	network, err := pool.CreateNetwork("test-on-start")
	s.Require().Nil(err)
	defer network.Close()
	s.chains = spinUpTestChains(s.T(), pool, network, testchains...)
	s.Require().Len(s.chains, len(testchains))
	s.T().Log("Network 1...", s.chains[0].resource.GetIPInNetwork(network))
	s.T().Log("Network 2...", s.chains[1].resource.GetIPInNetwork(network))
	// time.Sleep(10 * time.Minute)
	spinRelayer(s.T(), pool, network, s.chains[0].resource.GetIPInNetwork(network), s.chains[1].resource.GetIPInNetwork(network))
	// time.Sleep(15 * time.Second)
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
