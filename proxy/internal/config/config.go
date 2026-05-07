package config

import (
	"path/filepath"

	"github.com/spf13/viper"
)

type ServerConfig struct {
	Name      string `mapstructure:"name"`
	Transport string `mapstructure:"transport"`
	Binary    string `mapstructure:"binary"`
	Address   string `mapstructure:"address"`
	ConfigDir string `mapstructure:"-"` // populated at load time, not from yaml
}

type Config struct {
	Transport string         `mapstructure:"transport"`
	Port      int            `mapstructure:"port"`
	AdminPort int            `mapstructure:"admin_port"`
	Servers   []ServerConfig `mapstructure:"servers"`
}

func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetDefault("transport", "stdio")
	v.SetDefault("port", 8080)
	v.SetDefault("admin_port", 9090)

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	// Inject the config file's directory so transports can resolve relative paths.
	dir := filepath.Dir(path)
	for i := range cfg.Servers {
		cfg.Servers[i].ConfigDir = dir
	}
	return &cfg, nil
}
