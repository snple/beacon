package config

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

type ConfigStruct struct {
	Debug          bool        `toml:"debug"`
	DB             DB          `toml:"db"`
	CoreService    TCPService  `toml:"core"`
	NodeService    TCPService  `toml:"node"`
	TcpNodeService TCPService  `toml:"tcp_node"`
	Status         Status      `toml:"status"`
	Gin            Gin         `toml:"gin"`
	ApiService     HttpService `toml:"api"`
	Statics        []Static    `toml:"static"`
}

type DB struct {
	Debug bool   `toml:"debug"`
	File  string `toml:"file"`
}

type TCPService struct {
	Enable bool   `toml:"enable"`
	Addr   string `toml:"addr"`
	TLS    bool   `toml:"tls"`
	CA     string `toml:"ca"`
	Cert   string `toml:"cert"`
	Key    string `toml:"key"`
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

func DefaultConfig() ConfigStruct {
	return ConfigStruct{
		Debug: false,
		DB: DB{
			File: "store.db",
		},
		CoreService: TCPService{
			Enable: true,
			Addr:   ":6006",
			TLS:    true,
			CA:     "certs/ca.crt",
			Cert:   "certs/server.crt",
			Key:    "certs/server.key",
		},
		NodeService: TCPService{
			Enable: true,
			Addr:   ":6007",
			TLS:    true,
			CA:     "certs/ca.crt",
			Cert:   "certs/server.crt",
			Key:    "certs/server.key",
		},
		TcpNodeService: TCPService{
			Enable: true,
			Addr:   ":6008",
			TLS:    false,
		},
		Status: Status{
			LinkTTL: 3 * 60,
		},
		ApiService: HttpService{
			Addr: ":8008",
		},
	}
}

func (c *ConfigStruct) Validate() error {
	if c.DB.File == "" {
		return errors.New("DB.File must be specified")
	}

	if c.CoreService.Enable {
		if c.CoreService.Addr == "" {
			return errors.New("CoreService.Addr must be specified")
		}

		if c.CoreService.TLS {
			if c.CoreService.CA == "" {
				return errors.New("CoreService.CA must be specified")
			}

			if c.CoreService.Cert == "" {
				return errors.New("CoreService.Cert must be specified")
			}

			if c.CoreService.Key == "" {
				return errors.New("CoreService.Key must be specified")
			}
		}
	}

	if c.NodeService.Enable {
		if c.NodeService.Addr == "" {
			return errors.New("NodeService.Addr must be specified")
		}

		if c.NodeService.TLS {
			if c.NodeService.CA == "" {
				return errors.New("NodeService.CA must be specified")
			}

			if c.NodeService.Cert == "" {
				return errors.New("NodeService.Cert must be specified")
			}

			if c.NodeService.Key == "" {
				return errors.New("NodeService.Key must be specified")
			}
		}
	}

	return nil
}

var Config = DefaultConfig()

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
