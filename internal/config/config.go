package config

import (
	"net"
	"sync"
)

type Config struct {
	FirecrackerBinaryPath string
	GatewayIP             net.IP
	SubnetMask            string
}

var config *Config
var lock sync.RWMutex

func GetConfig() *Config {
	lock.RLock()
	if config != nil {
		defer lock.RUnlock()
		return config
	}
	lock.RUnlock()

	lock.Lock()
	defer lock.Unlock()
	if config != nil {
		return config
	}
	config = loadConfig()
	return config
}
func loadConfig() *Config {
	//Check well known locations and env if not return default
	//cache config to prevent expensive reload
	return LoadDefaultConfig()
}

func LoadDefaultConfig() *Config {
	return &Config{
		FirecrackerBinaryPath: "./vm/firecracker",
		GatewayIP:             net.ParseIP("172.102.0.1"),
		SubnetMask:            "255.255.0.0",
	}
}
