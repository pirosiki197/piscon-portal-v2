package config

import (
	"fmt"

	"github.com/spf13/viper"
)

func LoadFile() (Config, error) {
	viper.SetConfigName("piscon_runner")
	viper.AddConfigPath(".")

	if err := viper.ReadInConfig(); err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return Config{}, fmt.Errorf("unmarshal config: %w", err)
	}

	return cfg, nil
}

type Portal struct {
	Address string `mapstructure:"address"`
}

type Problem struct {
	Name    string         `mapstructure:"name"`
	Options map[string]any `mapstructure:"options"`
}

// Config は runner の設定である。問題は Bundle と Problem のどちらか一方で指定する。
type Config struct {
	Portal Portal `mapstructure:"portal"`
	// Bundle は、問題バンドルのディレクトリである。
	Bundle  string  `mapstructure:"bundle"`
	Problem Problem `mapstructure:"problem"`
}

func (c Config) Validate() error {
	if c.Portal.Address == "" {
		return fmt.Errorf("portal address is required")
	}
	if (c.Bundle == "") == (c.Problem.Name == "") {
		return fmt.Errorf("exactly one of bundle and problem.name is required")
	}

	return nil
}
