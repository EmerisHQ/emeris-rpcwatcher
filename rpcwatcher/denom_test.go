package rpcwatcher

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsPoolCoin(t *testing.T) {
	tests := []struct {
		name     string
		coin     string
		expValue bool
	}{
		{
			"coin with length less than 4",
			"new",
			false,
		},
		{
			"invalid pool coin",
			"testcoin",
			false,
		},
		{
			"valid pool coin",
			"pool96EF6EA6E5AC828ED87E8D07E7AE2A8180570ADD212117B2DA6F0B75D17A6295",
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value := isPoolCoin(tt.coin)
			require.Equal(t, tt.expValue, value)
		})
	}
}

func TestIsIBCToken(t *testing.T) {
	tests := []struct {
		name     string
		token    string
		expValue bool
	}{
		{
			"token with length less than 4",
			"new",
			false,
		},
		{
			"invalid ibc token",
			"testtoken",
			false,
		},
		{
			"valid ibc token",
			"ibc/B5CB286F69D48B2C4F6F8D8CF59011C40590DCF8A91617A5FBA9FF0A7B21307F",
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value := isIBCToken(tt.token)
			require.Equal(t, tt.expValue, value)
		})
	}
}
