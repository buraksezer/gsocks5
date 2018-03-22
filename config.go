// Copyright 2017 Burak Sezer
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	InsecureSkipVerify bool   `json:"insecure_skip_verify"`
	Role               string `json:"role"`
	ServerAddr         string `json:"server_addr"`
	ServerCert         string `json:"server_cert"`
	ServerKey          string `json:"server_key"`
	ClientAddr         string `json:"client_addr"`
	Password           string `json:"password"`
	Socks5Username     string `json:"socks5_username"`
	Socks5Password     string `json:"socks5_password"`
	KeepAlivePeriod    int    `json:"keepalive_period"`
	GracefulPeriod     int    `json:"graceful_period"`
	DialTimeout        int    `json:"dial_timeout"`
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
