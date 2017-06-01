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
)

type config struct {
	Debug          bool   `json:"debug"`
	Method         string `json:"method"`
	ServerHost     string `json:"server_host"`
	ServerPort     string `json:"server_port"`
	ClientHost     string `json:"client_host"`
	ClientPort     string `json:"client_port"`
	Password       string `json:"password"`
	Timeout        int    `json:"timeout"`
	GracefulPeriod int    `json:"graceful_period"`
	Role           string `json:"role"`
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
		cfg.GracefulPeriod = 5
	}
	return cfg, nil
}
