package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/gofiber/fiber/v2"

	"github.com/simplyzetax/aegis/internal/config"
	"github.com/simplyzetax/aegis/internal/dns"
	"github.com/simplyzetax/aegis/internal/platform"
	"github.com/simplyzetax/aegis/internal/proxy"
	"github.com/simplyzetax/aegis/internal/ssl"
	"github.com/simplyzetax/aegis/internal/ui"
)

func main() {
	// Load configuration
	if err := config.Load(); err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Show platform information
	log.Debugf("Platform: %s", platform.GetPlatform())
	log.Debugf("IsAdmin: %t", platform.IsAdmin())
	log.Debugf("CanEscalate: %t", platform.CanEscalate())
	log.Debugf("GetUserPrivilegeInfo: %s", platform.GetUserPrivilegeInfo())

	// Check for admin privileges
	if !platform.IsAdmin() {
		log.Warn("Aegis needs to be run as admin to function properly")
		if err := platform.EscalatePrivileges(); err != nil {
			log.Fatalf("Failed to escalate privileges: %v", err)
		}
	}

	if config.Config.SimpleMode.Enabled {
		log.Info("Starting Aegis in simple mode...")

		if config.Config.SimpleMode.Domain == "" {
			log.Fatal("Simple mode domain not configured. Please set simple_mode.domain in config.json")
		}

		if err := runSimpleMode(); err != nil {
			log.Fatalf("Simple mode failed: %v", err)
		}

		return
	}

	// Show startup menu
	for {
		action, err := ui.ShowStartupMenu()
		if err != nil {
			log.Fatalf("Menu error: %v", err)
		}

		switch action {
		case "start":
			if err := startProxyServer(); err != nil {
				log.Errorf("Failed to start proxy server: %v", err)
			}
		case "dns":
			if err := ui.DNSRedirectManagerForm(); err != nil {
				log.Errorf("DNS management error: %v", err)
			}
		case "cert":
			if err := manageCertificates(); err != nil {
				log.Errorf("Certificate management error: %v", err)
			}
		case "config":
			showConfiguration()
		case "exit":
			log.Info("Goodbye!")
			return
		default:
			log.Errorf("Unknown action: %s", action)
		}
	}
}

// startProxyServer starts the complete proxy server with DNS and HTTPS
func startProxyServer() error {
	log.Info("Starting Aegis proxy server...")

	// Ensure we have at least one certificate
	certs, err := ssl.ListCerts()
	if err != nil {
		return fmt.Errorf("failed to list certificates: %v", err)
	}

	var selectedCert string
	if len(certs) == 0 {
		log.Info("No certificates found. Please create one first.")
		return manageCertificates()
	} else if len(certs) == 1 {
		selectedCert = certs[0]
		log.Infof("Using certificate: %s", selectedCert)
	} else {
		// Show certificate selector
		if err := ui.CertSelectorForm(ssl.ListCerts, ssl.GenerateCerts); err != nil {
			return fmt.Errorf("certificate selection failed: %v", err)
		}
		selectedCert = ui.SelectedCert
	}

	// Validate the selected certificate
	if err := ssl.ValidateCert(selectedCert); err != nil {
		return fmt.Errorf("invalid certificate %s: %v", selectedCert, err)
	}

	// Start DNS server
	log.Info("Starting DNS server...")
	dnsPort, err := dns.StartService()
	if err != nil {
		return fmt.Errorf("failed to start DNS server: %v", err)
	}

	// Test DNS functionality
	if err := dns.TestDNSServer(dnsPort); err != nil {
		log.Warnf("DNS test failed: %v", err)
	}

	// Create and configure Fiber app
	app := fiber.New(fiber.Config{
		BodyLimit:       1024 * 1024 * 1024, // 1GB
		ReadBufferSize:  8096,
		WriteBufferSize: 8096,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			return c.JSON(fiber.Map{
				"code":    fiber.StatusInternalServerError,
				"message": "Internal Server Error",
				"error":   err.Error(),
			})
		},
		DisableStartupMessage: true,
	})

	// Set up the proxy handler
	app.All("*", proxy.Handler)

	// Show startup information
	log.Infof("🚀 Proxy server starting on port %s", config.Config.Proxy.Port)
	log.Infof("🔒 Using certificate: %s", selectedCert)
	log.Infof("⬆️  Upstream URL: %s", config.Config.Proxy.UpstreamURL)
	log.Infof("🌐 DNS server running on port %s", dnsPort)

	enabledRedirects := config.GetEnabledRedirects()
	if len(enabledRedirects) > 0 {
		log.Info("📋 Active DNS redirects:")
		for _, redirect := range enabledRedirects {
			log.Infof("   %s -> %s (%s)", redirect.Domain, redirect.Target, redirect.Description)
		}
	} else {
		log.Warn("⚠️  No DNS redirects configured!")
	}

	// Set up cleanup function
	defer func() {
		log.Info("Application shutting down...")
		dns.StopService()
	}()

	// Start HTTPS server
	address := ":" + config.Config.Proxy.Port
	log.Infof("✅ Server ready! Listening on https://localhost%s", address)
	return app.ListenTLSWithCertificate(address, ssl.LoadCert(selectedCert))
}

// manageCertificates handles certificate management
func manageCertificates() error {
	return ui.CertSelectorForm(ssl.ListCerts, ssl.GenerateCerts)
}

// showConfiguration displays current configuration
func showConfiguration() {
	log.Info("📄 Current Configuration:")
	log.Infof("   Log Level: %s", config.Config.LogLevel)
	log.Infof("   Proxy Upstream: %s", config.Config.Proxy.UpstreamURL)
	log.Infof("   Proxy Port: %s", config.Config.Proxy.Port)
	log.Infof("   DNS Upstream: %s", config.Config.DNS.UpstreamDNS)
	log.Infof("   DNS Auto-Manage: %t", config.Config.DNS.AutoManageSystem)
	log.Infof("   Proxy Headers: %v", config.Config.Proxy.Headers)

	log.Infof("   DNS Redirects (%d total):", len(config.Config.DNS.Redirects))
	for i, redirect := range config.Config.DNS.Redirects {
		status := "✅"
		if !redirect.Enabled {
			status = "❌"
		}
		log.Infof("     %d. %s %s -> %s (%s)", i+1, status, redirect.Domain, redirect.Target, redirect.Description)
	}

	// Show DNS service status if running
	if status := dns.GetServiceStatus(); status["running"].(bool) {
		log.Info("🌐 DNS Service Status:")
		log.Infof("   Port: %s", status["port"])
		log.Infof("   Active redirects: %d", status["enabled_count"])
		log.Infof("   Total redirects: %d", status["total_count"])
		log.Infof("   Upstream DNS: %s", status["upstream_dns"])
	} else {
		log.Info("🌐 DNS Service: Not running")
	}
}

// runSimpleMode handles the simple mode execution
func runSimpleMode() error {
	domain := config.Config.SimpleMode.Domain
	log.Infof("Simple mode domain: %s", domain)

	// Generate a safe certificate name for filesystem
	certName := strings.ReplaceAll(domain, "*", "_")

	// Check if certificate already exists
	certs, err := ssl.ListCerts()
	if err != nil {
		return fmt.Errorf("failed to list certificates: %v", err)
	}

	certExists := false
	for _, cert := range certs {
		if cert == certName {
			certExists = true
			break
		}
	}

	// Generate certificate if it doesn't exist
	if !certExists {
		log.Infof("Generating certificate for domain: %s", domain)
		if err := ssl.GenerateCerts(domain); err != nil {
			return fmt.Errorf("failed to generate certificate: %v", err)
		}
		log.Infof("Certificate generated for %s", domain)
	} else {
		log.Infof("Certificate for %s already exists", domain)
	}

	// Validate the certificate
	if err := ssl.ValidateCert(certName); err != nil {
		return fmt.Errorf("invalid certificate %s: %v", certName, err)
	}

	installed, err := ssl.IsCertificateInstalled(certName)
	if err != nil {
		return fmt.Errorf("failed to check if certificate is installed: %v", err)
	}

	if !installed {
		log.Infof("Certificate %s is not installed. Installing...", certName)
		if err := ssl.InstallCertificateToSystem(certName); err != nil {
			return fmt.Errorf("failed to install certificate: %v", err)
		}
		log.Infof("Certificate %s installed successfully", certName)
	}

	// Start DNS server
	log.Info("Starting DNS server...")
	dnsPort, err := dns.StartService()
	if err != nil {
		return fmt.Errorf("failed to start DNS server: %v", err)
	}

	// Test DNS functionality
	if err := dns.TestDNSServer(dnsPort); err != nil {
		log.Warnf("DNS test failed: %v", err)
	}

	// Create and configure Fiber app
	app := fiber.New(fiber.Config{
		BodyLimit:       1024 * 1024 * 1024, // 1GB
		ReadBufferSize:  8096,
		WriteBufferSize: 8096,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			return c.JSON(fiber.Map{
				"code":    fiber.StatusInternalServerError,
				"message": "Internal Server Error",
				"error":   err.Error(),
			})
		},
		DisableStartupMessage: true,
	})

	// Set up the proxy handler
	app.All("*", proxy.Handler)

	// Show startup information
	log.Infof("🚀 Simple Mode: Proxy server starting on port %s", config.Config.Proxy.Port)
	log.Infof("🔒 Using certificate: %s", certName)
	log.Infof("⬆️  Upstream URL: %s", config.Config.Proxy.UpstreamURL)
	log.Infof("🌐 DNS server running on port %s", dnsPort)
	log.Infof("📍 Domain: %s", domain)

	enabledRedirects := config.GetEnabledRedirects()
	if len(enabledRedirects) > 0 {
		log.Info("📋 Active DNS redirects:")
		for _, redirect := range enabledRedirects {
			log.Infof("   %s -> %s (%s)", redirect.Domain, redirect.Target, redirect.Description)
		}
	} else {
		log.Warn("⚠️  No DNS redirects configured!")
	}

	// Set up cleanup function
	defer func() {
		log.Info("Simple mode shutting down...")
		dns.StopService()
	}()

	// Start HTTPS server
	address := ":" + config.Config.Proxy.Port
	log.Infof("✅ Simple Mode ready! Listening on https://localhost%s", address)
	log.Infof("💡 Point your applications to use DNS server 127.0.0.1:%s", strings.TrimPrefix(dnsPort, ":"))
	return app.ListenTLSWithCertificate(address, ssl.LoadCert(certName))
}
