package rpcwatcher

import (
	"testing"

	"github.com/stretchr/testify/require"
)

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
