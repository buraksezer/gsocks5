package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
)

const (
	roleClient = "client"
	roleServer = "server"
	// KeepAlive period for TCP sockets, in seconds.
	defaultTCPKeepAlivePeriod = 3600
	defaultGracefulPeriod     = 10
	defaultDialTimeout        = 10
)

type config struct {
	Debug              bool   `json:"debug"`
	ServerHost         string `json:"server_host"`
	ServerPort         string `json:"server_port"`
	InsecureSkipVerify bool   `json:"insecure_skip_verify"`
	ServerCert         string `json:"server_cert"`
	ServerKey          string `json:"server_key"`
	ClientHost         string `json:"client_host"`
	ClientPort         string `json:"client_port"`
	Password           string `json:"password"`
	KeepAlivePeriod    int    `json:"keepalive_period"`
	GracefulPeriod     int    `json:"graceful_period"`
	DialTimeout        int    `json:"dial_timeout"`
	Role               string `json:"role"`
}

func newConfig(file string) (config, error) {
	var cfg config
	if _, err := os.Stat(file); os.IsNotExist(err) {
		return cfg, errors.New("Configuration file cannot be found")
	}

	data, err := ioutil.ReadFile(file)
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	// Default value is 10. It's a random number.
	if cfg.GracefulPeriod == 0 {
		cfg.GracefulPeriod = defaultGracefulPeriod
	}

	if cfg.KeepAlivePeriod == 0 {
		cfg.KeepAlivePeriod = defaultTCPKeepAlivePeriod
	}

	if cfg.DialTimeout == 0 {
		cfg.DialTimeout = defaultDialTimeout
	}

	if cfg.Role == roleServer {
		if cfg.ServerCert == "" {
			return cfg, errors.New("server_cert cannot be empty")
		}

		if cfg.ServerKey == "" {
			return cfg, errors.New("server_key cannot be empty")
		}
	}
	return cfg, nil
}
