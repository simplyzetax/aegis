package core

import (
	"github.com/charmbracelet/log"
	"github.com/spf13/viper"
)

type AppConfig struct {
	UpstreamURL string `mapstructure:"upstream_url"`
}

var Config AppConfig

func LoadConfig() {
	viper.SetConfigFile("config.json")
	viper.SetConfigType("json")
	viper.AddConfigPath(".")

	if err := viper.ReadInConfig(); err != nil {
		viper.Set("upstream_url", "http://localhost:8787")
		viper.WriteConfig()
		log.Fatalf("failed to read config: %v", err)
	}

	viper.Unmarshal(&Config)
}
