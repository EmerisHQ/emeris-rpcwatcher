package integration

import (
	"fmt"
	"testing"

	"github.com/cosmos/cosmos-sdk/types/rest"
	"github.com/stretchr/testify/require"
)

func TestSetup(t *testing.T) {
	chains := spinUpTestChains(t, gaiaTestChain, akashTestChain)
	require.Len(t, chains, 2)

	response, err := rest.GetRequest(fmt.Sprintf("http://localhost:%s/status", chains[0].rpcPort))

	require.NoError(t, err)

	t.Log(string(response))

	response2, err := rest.GetRequest(fmt.Sprintf("http://localhost:%s/status", chains[1].rpcPort))

	require.NoError(t, err)

	t.Log(string(response2))

	require.True(t, false)
}
