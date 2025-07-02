package config

// DNSRedirect represents a single DNS redirect configuration
type DNSRedirect struct {
	Domain      string `json:"domain" mapstructure:"domain"`           // Domain pattern (e.g., "*.ol.epicgames.com")
	Target      string `json:"target" mapstructure:"target"`           // Target IP (usually "127.0.0.1")
	Description string `json:"description" mapstructure:"description"` // User-friendly description
	Enabled     bool   `json:"enabled" mapstructure:"enabled"`         // Whether this redirect is active
}

// DNSConfig holds DNS server configuration
type DNSConfig struct {
	Redirects        []DNSRedirect `json:"redirects" mapstructure:"redirects"`
	UpstreamDNS      string        `json:"upstream_dns" mapstructure:"upstream_dns"`
	Port             string        `json:"port" mapstructure:"port"`
	AutoManageSystem bool          `json:"auto_manage_system" mapstructure:"auto_manage_system"`
}

// ProxyConfig holds proxy server configuration
type ProxyConfig struct {
	UpstreamURL string            `json:"upstream_url" mapstructure:"upstream_url"`
	Port        string            `json:"port" mapstructure:"port"`
	Headers     map[string]string `json:"headers" mapstructure:"headers"` // Custom headers to inject into requests
}

// SimpleModeConfig holds simple mode configuration
type SimpleModeConfig struct {
	Enabled bool   `json:"enabled" mapstructure:"enabled"`
	Domain  string `json:"domain" mapstructure:"domain"` // Domain for the certificate (e.g., "localhost", "*.example.com")
}

// AppConfig represents the complete application configuration
type AppConfig struct {
	LogLevel   string           `json:"log_level" mapstructure:"log_level"`
	DNS        DNSConfig        `json:"dns" mapstructure:"dns"`
	Proxy      ProxyConfig      `json:"proxy" mapstructure:"proxy"`
	SimpleMode SimpleModeConfig `json:"simple_mode" mapstructure:"simple_mode"`
}

// GetDefaultConfig returns a configuration with sensible defaults
func GetDefaultConfig() *AppConfig {
	return &AppConfig{
		LogLevel: "info",
		DNS: DNSConfig{
			Redirects: []DNSRedirect{
				{
					Domain:      "*.ol.epicgames.com",
					Target:      "127.0.0.1",
					Description: "Epic Games Online Services",
					Enabled:     true,
				},
			},
			UpstreamDNS:      "1.1.1.1:53",
			Port:             "53",
			AutoManageSystem: true,
		},
		Proxy: ProxyConfig{
			UpstreamURL: "http://localhost:8787",
			Port:        "443",
			Headers: map[string]string{
				"X-Telemachus-Identifier": "",
			},
		},
		SimpleMode: SimpleModeConfig{
			Enabled: true,
			Domain:  "*.ol.epicgames.com",
		},
	}
}
