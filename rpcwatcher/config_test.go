package rpcwatcher

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	configName = "rpcwatcher"
	testDBURL  = "postgres://root:unused@?host=%2Ftmp%2Fdemo174156101&port=26257"
)

func TestReadConfig(t *testing.T) {
	tests := []struct {
		name        string
		env         map[string]string
		expectedCfg *Config
		expErr      bool
	}{
		{
			"config with default values : missing db connection url",
			map[string]string{},
			&Config{
				RedisURL:           defaultRedisURL,
				ApiURL:             defaultApiURL,
				ProfilingServerURL: defaultProfilingServerURL,
			},
			true,
		},
		{
			"set env with invalid redis url",
			map[string]string{
				"DatabaseConnectionURL": testDBURL,
				"RedisURL":              "http://redis-server:1234",
			},
			&Config{
				DatabaseConnectionURL: testDBURL,
				RedisURL:              "http://redis-server:1234",
				ApiURL:                defaultApiURL,
				ProfilingServerURL:    defaultProfilingServerURL,
			},
			true,
		},
		{
			"set env with invalid api url",
			map[string]string{
				"DatabaseConnectionURL": testDBURL,
				"ApiURL":                "0.0.0.0:3456",
			},
			&Config{
				DatabaseConnectionURL: testDBURL,
				RedisURL:              defaultRedisURL,
				ApiURL:                "0.0.0.0:3456",
				ProfilingServerURL:    defaultProfilingServerURL,
			},
			true,
		},
		{
			"set env with invalid profiling server url",
			map[string]string{
				"DatabaseConnectionURL": testDBURL,
				"ProfilingServerURL":    "http://profiling-server:1234",
			},
			&Config{
				DatabaseConnectionURL: testDBURL,
				RedisURL:              defaultRedisURL,
				ApiURL:                defaultApiURL,
				ProfilingServerURL:    "http://profiling-server:1234",
			},
			true,
		},
		{
			"valid config with default values",
			map[string]string{
				"DatabaseConnectionURL": testDBURL,
			},
			&Config{
				DatabaseConnectionURL: testDBURL,
				RedisURL:              defaultRedisURL,
				ApiURL:                defaultApiURL,
				ProfilingServerURL:    defaultProfilingServerURL,
				Debug:                 false,
				JSONLogs:              false,
			},
			false,
		},
		{
			"valid config modified with env values",
			map[string]string{
				"DatabaseConnectionURL": testDBURL,
				"RedisURL":              "0.0.0.0:6379",
				"ApiURL":                "http://0.0.0.0:8080",
				"ProfilingServerURL":    ":7777",
				"Debug":                 "true",
				"JSONLogs":              "true",
			},
			&Config{
				DatabaseConnectionURL: testDBURL,
				RedisURL:              "0.0.0.0:6379",
				ApiURL:                "http://0.0.0.0:8080",
				ProfilingServerURL:    ":7777",
				Debug:                 true,
				JSONLogs:              true,
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				require.NoError(t, os.Setenv(
					strings.ToUpper(fmt.Sprintf("%s_%s", configName, k)),
					v,
				),
				)
			}

			defer func() {
				os.Clearenv()
			}()

			readCfg, err := ReadConfig()
			if tt.expErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.expectedCfg, readCfg)
		})
	}
}
