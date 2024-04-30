package config

type Config struct {
	FirecrackerBinaryPath string
}

func LoadConfig() (*Config, error) {
	return &Config{
		FirecrackerBinaryPath: "/usr/local/bin/firecracker",
	}, nil
}

func LoadDefaultConfig() *Config {
	return &Config{
		FirecrackerBinaryPath: "./vm/firecracker",
	}
}
