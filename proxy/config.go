package proxy

import (
	"github.com/hostup/open-zano-pool/api"
	"github.com/hostup/open-zano-pool/exchange"
	"github.com/hostup/open-zano-pool/payouts"
	"github.com/hostup/open-zano-pool/policy"
	"github.com/hostup/open-zano-pool/storage"
)

type Config struct {
	Name                  string        `json:"name"`
	Proxy                 Proxy         `json:"proxy"`
	Api                   api.ApiConfig `json:"api"`
	Upstream              []Upstream    `json:"upstream"`
	UpstreamCheckInterval string        `json:"upstreamCheckInterval"`

	Threads int `json:"threads"`

	Coin     string         `json:"coin"`
	Pplns    int64          `json:"pplns"`
	CoinName string         `json:"coin-name"`
	Redis    storage.Config `json:"redis"`

	BlockUnlocker payouts.UnlockerConfig `json:"unlocker"`
	Payouts       payouts.PayoutsConfig  `json:"payouts"`

	Exchange exchange.ExchangeConfig `json:"exchange"`

	NewrelicName    string `json:"newrelicName"`
	NewrelicKey     string `json:"newrelicKey"`
	NewrelicVerbose bool   `json:"newrelicVerbose"`
	NewrelicEnabled bool   `json:"newrelicEnabled"`
}

type Proxy struct {
	Enabled              bool   `json:"enabled"`
	Listen               string `json:"listen"`
	LimitHeadersSize     int    `json:"limitHeadersSize"`
	LimitBodySize        int64  `json:"limitBodySize"`
	BehindReverseProxy   bool   `json:"behindReverseProxy"`
	BlockRefreshInterval string `json:"blockRefreshInterval"`
	Difficulty           int64  `json:"difficulty"`
	StateUpdateInterval  string `json:"stateUpdateInterval"`
	HashrateExpiration   string `json:"hashrateExpiration"`
  Address              string `json:"address"`
	Policy policy.Config `json:"policy"`

	MaxFails    int64 `json:"maxFails"`
	HealthCheck bool  `json:"healthCheck"`

	Stratum    Stratum    `json:"stratum"`
	StratumSSL StratumSSL `json:"stratum_ssl"`

	StratumNiceHash StratumNiceHash `json:"stratum_nice_hash"`
}

type Stratum struct {
	Enabled  bool   `json:"enabled"`
	Listen   string `json:"listen"`
	Timeout  string `json:"timeout"`
	MaxConn  int    `json:"maxConn"`
	TLS      bool   `json:"tls"`
	CertFile string `json:"certfile"`
	KeyFile  string `json:"certkey"`
}

type StratumSSL struct {
	Enabled  bool   `json:"enabled"`
	Listen   string `json:"listen"`
	Timeout  string `json:"timeout"`
	MaxConn  int    `json:"maxConn"`
	CertFile string `json:"certfile"`
	CertKey  string `json:"certkey"`
}

type StratumNiceHash struct {
	Enabled bool   `json:"enabled"`
	Listen  string `json:"listen"`
	Timeout string `json:"timeout"`
	MaxConn int    `json:"maxConn"`
}

type Upstream struct {
	Name    string `json:"name"`
	Url     string `json:"url"`
	Timeout string `json:"timeout"`
}
