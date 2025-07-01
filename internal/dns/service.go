package dns

import (
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	"github.com/miekg/dns"
	"github.com/simplyzetax/aegis/internal/config"
)

// Service manages both the DNS server and system DNS settings
type Service struct {
	server  *Server
	manager *Manager
	port    string
}

// globalDNSService tracks the DNS service instance
var globalDNSService *Service

// NewService creates a new DNS service instance
func NewService() *Service {
	return &Service{
		server:  NewServer(),
		manager: NewManager(),
	}
}

// StartService starts the DNS server and configures system DNS
func StartService() (string, error) {
	globalDNSService = NewService()

	// Get current DNS settings first
	log.Info("Getting current DNS settings...")
	if err := globalDNSService.manager.GetCurrentDNS(); err != nil {
		log.Warnf("Failed to get current DNS settings: %v", err)
		log.Info("Continuing without DNS management...")
	} else {
		if len(globalDNSService.manager.GetOriginalDNS()) > 0 {
			log.Info("Current DNS settings saved")
			// Set up signal handlers for graceful cleanup
			globalDNSService.manager.SetupSignalHandlers()
		} else {
			log.Info("No manageable network interfaces found")
			log.Info("Continuing without DNS management...")
		}
	}

	// Try to start DNS server on various ports
	ports := []string{":53", ":8053", ":5353", ":9053", ":10053"}

	var lastErr error
	for _, port := range ports {
		log.Debugf("Trying to start DNS server on port %s", port)
		if err := globalDNSService.server.Start(port); err != nil {
			lastErr = err
			log.Debugf("Port %s failed: %v", port, err)
			continue
		}

		globalDNSService.port = port
		log.Infof("DNS server successfully started on port %s", port)

		// Configure system DNS if we have original settings and auto-manage is enabled
		if config.Config.DNS.AutoManageSystem && len(globalDNSService.manager.GetOriginalDNS()) > 0 {
			log.Info("Configuring system DNS to use local DNS server...")
			if err := globalDNSService.manager.SetDNSToLocal(port); err != nil {
				log.Warnf("Failed to configure system DNS: %v", err)
				log.Infof("You may need to manually configure DNS to use 127.0.0.1")
			} else {
				log.Info("System DNS configured successfully")
			}
		} else {
			log.Info("DNS management disabled or unavailable - manually configure DNS to use 127.0.0.1")
		}

		return port, nil
	}

	return "", fmt.Errorf("failed to start DNS server on any port: %v", lastErr)
}

// StopService stops the DNS server and restores original DNS settings
func StopService() error {
	if globalDNSService == nil {
		return nil
	}

	var serverErr, managerErr error

	// Stop the DNS server
	if globalDNSService.server != nil {
		serverErr = globalDNSService.server.Stop()
	}

	// Restore original DNS settings if auto-manage is enabled
	if config.Config.DNS.AutoManageSystem && globalDNSService.manager != nil {
		managerErr = globalDNSService.manager.RestoreOriginalDNS()
	}

	if serverErr != nil {
		return serverErr
	}
	return managerErr
}

// TestDNSServer tests the DNS server functionality
func TestDNSServer(port string) error {
	if globalDNSService == nil {
		return fmt.Errorf("DNS service not started")
	}

	// Create a test DNS client
	client := new(dns.Client)
	client.Timeout = 2 * time.Second

	// Test with a configured redirect (if any exist)
	redirects := config.GetEnabledRedirects()
	if len(redirects) == 0 {
		log.Info("No DNS redirects configured - skipping test")
		return nil
	}

	// Use the first redirect for testing
	testDomain := redirects[0].Domain
	if testDomain[0] == '*' && len(testDomain) > 2 {
		// For wildcard domains, create a test subdomain
		testDomain = "test" + testDomain[1:]
	}

	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(testDomain), dns.TypeA)

	// Use the actual server address
	serverAddr := fmt.Sprintf("127.0.0.1%s", port)
	resp, _, err := client.Exchange(m, serverAddr)
	if err != nil {
		return fmt.Errorf("DNS test failed: %v", err)
	}

	if len(resp.Answer) == 0 {
		return fmt.Errorf("no DNS answer received for test domain %s", testDomain)
	}

	// Check if we got the expected redirect response
	if a, ok := resp.Answer[0].(*dns.A); ok {
		expectedIP := redirects[0].Target
		if a.A.String() == expectedIP {
			log.Debugf("DNS test successful: %s redirected to %s", testDomain, expectedIP)
			return nil
		} else {
			return fmt.Errorf("unexpected DNS response: got %s, expected %s", a.A.String(), expectedIP)
		}
	}

	return fmt.Errorf("unexpected DNS response type")
}

// ReloadRedirects reloads the DNS redirects configuration
func ReloadRedirects() error {
	if globalDNSService == nil || globalDNSService.server == nil {
		return fmt.Errorf("DNS service not started")
	}

	globalDNSService.server.ReloadRedirects()
	return nil
}

// GetServiceStatus returns comprehensive status information about the DNS service
func GetServiceStatus() map[string]interface{} {
	if globalDNSService == nil {
		return map[string]interface{}{
			"running": false,
			"error":   "DNS service not started",
		}
	}

	status := map[string]interface{}{
		"running":      true,
		"port":         globalDNSService.port,
		"auto_manage":  config.Config.DNS.AutoManageSystem,
		"upstream_dns": config.Config.DNS.UpstreamDNS,
	}

	// Add server status
	if globalDNSService.server != nil {
		serverStatus := globalDNSService.server.GetRedirectStatus()
		for k, v := range serverStatus {
			status[k] = v
		}
	}

	// Add manager status
	if globalDNSService.manager != nil {
		status["original_dns_count"] = len(globalDNSService.manager.GetOriginalDNS())
		status["original_dns"] = globalDNSService.manager.GetOriginalDNS()
	}

	return status
}

// ResetAllDNSToAuto is a utility function to reset all DNS settings
func ResetAllDNSToAuto() error {
	manager := NewManager()
	return manager.ResetAllDNSToAuto()
}
