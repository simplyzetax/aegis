package config

import (
	"fmt"
	"os"

	"github.com/charmbracelet/log"
	"github.com/spf13/viper"
)

var Config *AppConfig

// Load reads the configuration from file or creates default config
func Load() error {
	viper.SetConfigFile("config.json")
	viper.SetConfigType("json")
	viper.AddConfigPath(".")

	// Try to read existing config
	if err := viper.ReadInConfig(); err != nil {
		log.Info("No existing config found, creating default configuration...")

		// Set default configuration
		Config = GetDefaultConfig()

		// Save the default config to file
		if err := Save(); err != nil {
			return fmt.Errorf("failed to create default config file: %v", err)
		}

		log.Info("Default configuration created in config.json")
		return nil
	}

	// Unmarshal config from file
	Config = &AppConfig{}
	if err := viper.Unmarshal(Config); err != nil {
		return fmt.Errorf("failed to unmarshal config: %v", err)
	}

	// Validate configuration
	if err := validate(); err != nil {
		return fmt.Errorf("invalid configuration: %v", err)
	}

	setLogLevel(Config.LogLevel)
	log.Debug("Configuration loaded successfully")
	return nil
}

// Save writes the current configuration to file
func Save() error {
	// Convert config to viper format
	viper.Set("log_level", Config.LogLevel)
	viper.Set("dns", Config.DNS)
	viper.Set("proxy", Config.Proxy)

	return viper.WriteConfig()
}

// AddRedirect adds a new DNS redirect to the configuration
func AddRedirect(redirect DNSRedirect) error {
	Config.DNS.Redirects = append(Config.DNS.Redirects, redirect)
	return Save()
}

// RemoveRedirect removes a DNS redirect by index
func RemoveRedirect(index int) error {
	if index < 0 || index >= len(Config.DNS.Redirects) {
		return fmt.Errorf("invalid redirect index: %d", index)
	}

	Config.DNS.Redirects = append(
		Config.DNS.Redirects[:index],
		Config.DNS.Redirects[index+1:]...,
	)
	return Save()
}

// ToggleRedirect enables/disables a DNS redirect by index
func ToggleRedirect(index int) error {
	if index < 0 || index >= len(Config.DNS.Redirects) {
		return fmt.Errorf("invalid redirect index: %d", index)
	}

	Config.DNS.Redirects[index].Enabled = !Config.DNS.Redirects[index].Enabled
	return Save()
}

// GetEnabledRedirects returns only the enabled DNS redirects
func GetEnabledRedirects() []DNSRedirect {
	var enabled []DNSRedirect
	for _, redirect := range Config.DNS.Redirects {
		if redirect.Enabled {
			enabled = append(enabled, redirect)
		}
	}
	return enabled
}

// validate checks if the configuration is valid
func validate() error {

	if Config.DNS.UpstreamDNS == "" {
		Config.DNS.UpstreamDNS = "1.1.1.1:53"
	}

	if Config.Proxy.UpstreamURL == "" {
		return fmt.Errorf("proxy upstream_url is required")
	}

	// Validate DNS redirects
	for i, redirect := range Config.DNS.Redirects {
		if redirect.Domain == "" {
			return fmt.Errorf("redirect %d: domain is required", i)
		}
		if redirect.Target == "" {
			return fmt.Errorf("redirect %d: target is required", i)
		}
	}

	return nil
}

// setLogLevel configures the log level
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

// Reload reloads the configuration from file
func Reload() error {
	return Load()
}

// GetConfigPath returns the path to the configuration file
func GetConfigPath() string {
	return "config.json"
}

// BackupConfig creates a backup of the current configuration
func BackupConfig() error {
	configPath := GetConfigPath()
	backupPath := configPath + ".backup"

	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %v", err)
	}

	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write backup file: %v", err)
	}

	log.Debugf("Configuration backed up to %s", backupPath)
	return nil
}
