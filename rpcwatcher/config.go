package rpcwatcher

import (
	"github.com/allinbits/demeris-backend-models/validation"
	"github.com/allinbits/emeris-rpcwatcher/utils/configuration"
	"github.com/go-playground/validator/v10"
)

type Config struct {
	DatabaseConnectionURL string `validate:"required"`
	RedisURL              string `validate:"required,hostname_port"`
	ApiURL                string `validate:"required,url"`
	ProfilingServerURL    string `validate:"hostname_port"`
	Debug                 bool
	JSONLogs              bool
}

func (c *Config) Validate() error {
	err := validator.New().Struct(c)
	if err == nil {
		return nil
	}

	return validation.MissingFieldsErr(err, false)
}

func ReadConfig() (*Config, error) {
	var c Config
	return &c, configuration.ReadConfig(&c, "rpcwatcher", map[string]string{
		"RedisURL":           "redis-master:6379",
		"ApiURL":             "http://api-server:8000",
		"ProfilingServerURL": ":6060",
	})
}
