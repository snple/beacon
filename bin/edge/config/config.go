package config

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

type ConfigStruct struct {
	Debug       bool        `toml:"debug"`
	NodeID      string      `toml:"node_id"`
	Secret      string      `toml:"secret"`
	DB          DB          `toml:"db"`
	BadgerDB    BadgerDB    `toml:"badger"`
	NodeClient  TCPClient   `toml:"node"`
	EdgeService TCPService  `toml:"edge"`
	Sync        Sync        `toml:"sync"`
	Status      Status      `toml:"status"`
	Gin         Gin         `toml:"gin"`
	ApiService  HttpService `toml:"api"`
	Statics     []Static    `toml:"static"`
}

type DB struct {
	Debug bool   `toml:"debug"`
	File  string `toml:"file"`
}

type BadgerDB struct {
	Path     string `toml:"path"`
	InMemory bool   `toml:"in_memory"`
}

type TCPClient struct {
	Enable             bool   `toml:"enable"`
	Addr               string `toml:"addr"`
	TLS                bool   `toml:"tls"`
	CA                 string `toml:"ca"`
	Cert               string `toml:"cert"`
	Key                string `toml:"key"`
	ServerName         string `toml:"server_name"`
	InsecureSkipVerify bool   `toml:"insecure_skip_verify"`
}

type TCPService struct {
	Enable bool   `toml:"enable"`
	Addr   string `toml:"addr"`
	TLS    bool   `toml:"tls"`
	CA     string `toml:"ca"`
	Cert   string `toml:"cert"`
	Key    string `toml:"key"`
}

type Sync struct {
	TokenRefresh int  `toml:"token_refresh"`
	Link         int  `toml:"link"`
	Interval     int  `toml:"interval"`
	Realtime     bool `toml:"realtime"`
}

type Status struct {
	LinkTTL int `toml:"link_ttl"`
}

type Gin struct {
	Debug bool `toml:"debug"`
}

type HttpService struct {
	Enable bool   `toml:"enable"`
	Debug  bool   `toml:"debug"`
	Addr   string `toml:"addr"`
	TLS    bool   `toml:"tls"`
	CA     string `toml:"ca"`
	Cert   string `toml:"cert"`
	Key    string `toml:"key"`
}

type Static struct {
	Enable bool   `toml:"enable"`
	Addr   string `toml:"addr"`
	Path   string `toml:"path"`
	TLS    bool   `toml:"tls"`
	Cert   string `toml:"cert"`
	Key    string `toml:"key"`
}

type Slim struct {
	Enable   bool   `toml:"enable"`
	Interval int    `toml:"interval"`
	BBolt    string `toml:"bbolt"`
	CacheTTL int    `toml:"cache_ttl"`
}

func DefaultConfig() ConfigStruct {
	return ConfigStruct{
		Debug: false,
		DB: DB{
			File: "store.db",
		},
		BadgerDB: BadgerDB{
			Path:     "badger",
			InMemory: true,
		},
		NodeClient: TCPClient{
			Addr: "127.0.0.1:6007",
			TLS:  true,
			CA:   "certs/ca.crt",
			Cert: "certs/client.crt",
			Key:  "certs/client.key",
		},
		EdgeService: TCPService{
			Addr: "127.0.0.1:6010",
			TLS:  true,
			CA:   "certs/ca.crt",
			Cert: "certs/server.crt",
			Key:  "certs/server.key",
		},
		Sync: Sync{
			TokenRefresh: 60 * 30,
			Link:         60,
			Interval:     60,
			Realtime:     false,
		},
		Status: Status{
			LinkTTL: 3 * 60,
		},
		ApiService: HttpService{
			Addr: ":8012",
		},
	}
}

var Config = DefaultConfig()

func (c *ConfigStruct) Validate() error {
	if len(c.NodeID) == 0 {
		return errors.New("NodeID must be specified")
	}

	if len(c.Secret) == 0 {
		return errors.New("Secret must be specified")
	}

	{
		if len(c.NodeClient.Addr) == 0 {
			return errors.New("NodeClient.Addr must be specified")
		}

		if c.NodeClient.TLS {
			if len(c.NodeClient.CA) == 0 {
				return errors.New("NodeClient.CA must be specified")
			}

			if len(c.NodeClient.Cert) == 0 {
				return errors.New("NodeClient.Cert must be specified")
			}

			if len(c.NodeClient.Key) == 0 {
				return errors.New("NodeClient.Key must be specified")
			}
		}
	}

	if c.EdgeService.Enable {
		if len(c.EdgeService.Addr) == 0 {
			return errors.New("EdgeService.Addr must be specified")
		}

		if c.EdgeService.TLS {
			if len(c.EdgeService.CA) == 0 {
				return errors.New("EdgeService.CA must be specified")
			}

			if len(c.EdgeService.Cert) == 0 {
				return errors.New("EdgeService.Cert must be specified")
			}

			if len(c.EdgeService.Key) == 0 {
				return errors.New("EdgeService.Key must be specified")
			}
		}
	}

	return nil
}

func Parse() {
	var err error

	configFile := flag.String("c", "config.toml", "config file")
	flag.Parse()

	if _, err = toml.DecodeFile(*configFile, &Config); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if err = Config.Validate(); err != nil {
		fmt.Println("config:", err)
		os.Exit(1)
	}
}
