package config

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
)

var (
	ErrVirtualServerDuplicated   = errors.New("Vritual Server Duplicated")
	ErrPoolMemberDuplicated      = errors.New("Pool Member Duplicated")
	ErrVirtualServerNameEmpty    = errors.New("Vritual Server Name is not specified")
	ErrVirtualServerAddressEmpty = errors.New("Vritual Server Address is not specified")
)

type Server struct {
	Address string `json:"address"`
	Weight  int    `json:"weight"`
}

type VirtualServer struct {
	Name       string   `json:"name"`
	Address    string   `json:"address"`
	ServerName string   `json:"server_name"`
	Protocol   string   `json:"protocol"`
	CertFile   string   `json:"cert_file"`
	KeyFile    string   `json:"key_file"`
	LBMethod   string   `json:"lb_method"`
	Pool       []Server `json:"pool"`
}

type Authentication struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type Controller struct {
	Address string         `json:"address"`
	Auth    Authentication `json:"auth"`
}

type ServiceDiscovery struct {
	Type          string `json:"type"`
	Cluster       string `json:"cluster"`
	Prefix        string `json:"prefix"`
	CertFile      string `json:"cert_file"`
	KeyFile       string `json:"key_file"`
	TrustedCAFile string `json:"trusted_ca_file"`
}

type Configuration struct {
	ServiceDiscovery ServiceDiscovery `json:"service_discovery"`
	Controller       Controller       `json:"controller"`
	VServers         []VirtualServer  `json:"virtual_server"`
}

func Load(configFile string) (*Configuration, error) {
	file, err := os.Open(configFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	c := &Configuration{}
	decoder := json.NewDecoder(file)
	if err = decoder.Decode(c); err != nil {
		return nil, err
	}
	if err = c.check(); err != nil {
		return nil, err
	}
	return c, nil
}

func LoadFromString(config string) (*Configuration, error) {
	var err error
	c := &Configuration{}
	decoder := json.NewDecoder(strings.NewReader(config))
	if err = decoder.Decode(c); err != nil {
		return nil, err
	}
	if err = c.check(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Configuration) check() error {
	set := make(map[string]bool)
	for _, vs := range c.VServers {
		if vs.Name == "" {
			return ErrVirtualServerNameEmpty
		}

		if vs.Address == "" {
			return ErrVirtualServerAddressEmpty
		}

		if _, ok := set[vs.Name]; ok {
			return ErrVirtualServerDuplicated
		} else {
			set[vs.Name] = true
		}

		if len(vs.Pool) > 1 {
			pset := make(map[string]bool)
			for _, p := range vs.Pool {
				if _, ok := pset[p.Address]; ok {
					return ErrPoolMemberDuplicated
				} else {
					pset[p.Address] = true
				}
			}
		}
	}
	return nil
}
