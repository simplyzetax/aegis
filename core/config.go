package core

import (
	"github.com/charmbracelet/log"
	"github.com/spf13/viper"
)

type AppConfig struct {
	UpstreamURL string `mapstructure:"upstream_url"`
	Identifier  string `mapstructure:"identifier"`
	LogLevel    string `mapstructure:"log_level"`
}

var Config AppConfig

func LoadConfig() {
	viper.SetConfigFile("config.json")
	viper.SetConfigType("json")
	viper.AddConfigPath(".")

	if err := viper.ReadInConfig(); err != nil {
		// Set default values and create config file
		viper.Set("upstream_url", "http://localhost:8787")
		viper.Set("identifier", "")
		viper.Set("log_level", "info")

		if writeErr := viper.WriteConfig(); writeErr != nil {
			log.Fatalf("failed to create config file: %v", writeErr)
		}
		log.Fatalf("failed to read config: %v", err)
	}

	if err := viper.Unmarshal(&Config); err != nil {
		log.Fatalf("failed to unmarshal config: %v", err)
	}

	if Config.Identifier == "" {
		log.Fatal("identifier is required")
	}

	if Config.LogLevel == "" {
		Config.LogLevel = "info"
	}

	setLogLevel(Config.LogLevel)
}

func setLogLevel(level string) {
	switch level {
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "info":
		log.SetLevel(log.InfoLevel)
	case "warn":
		log.SetLevel(log.WarnLevel)
	case "error":
		log.SetLevel(log.ErrorLevel)
	default:
		log.SetLevel(log.InfoLevel)
	}
}
