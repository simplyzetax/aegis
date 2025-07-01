package dns

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	"github.com/charmbracelet/log"
)

// Manager handles system DNS configuration across platforms
type Manager struct {
	originalDNS map[string][]string // interface -> DNS servers
	ourDNSPort  string              // the port our DNS server is using
	platform    string
}

// NewManager creates a new DNS manager instance
func NewManager() *Manager {
	return &Manager{
		originalDNS: make(map[string][]string),
		platform:    runtime.GOOS,
	}
}

// GetCurrentDNS retrieves current DNS settings for all active interfaces
func (dm *Manager) GetCurrentDNS() error {
	switch dm.platform {
	case "windows":
		return dm.getCurrentDNSWindows()
	case "darwin":
		return dm.getCurrentDNSMacOS()
	default:
		return fmt.Errorf("unsupported platform: %s", dm.platform)
	}
}

// SetDNSToLocal configures system DNS to use our local DNS server
func (dm *Manager) SetDNSToLocal(port string) error {
	dm.ourDNSPort = port
	localDNS := "127.0.0.1" // Only IP address, no port for DNS configuration

	switch dm.platform {
	case "windows":
		return dm.setDNSWindows(localDNS)
	case "darwin":
		return dm.setDNSMacOS(localDNS)
	default:
		return fmt.Errorf("unsupported platform: %s", dm.platform)
	}
}

// RestoreOriginalDNS restores the original DNS settings
func (dm *Manager) RestoreOriginalDNS() error {
	log.Info("Restoring original DNS settings...")

	switch dm.platform {
	case "windows":
		return dm.restoreDNSWindows()
	case "darwin":
		return dm.restoreDNSMacOS()
	default:
		return fmt.Errorf("unsupported platform: %s", dm.platform)
	}
}

// Windows-specific implementations
func (dm *Manager) getCurrentDNSWindows() error {
	cmd := exec.Command("netsh", "interface", "ipv4", "show", "dnsservers")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get Windows DNS settings: %v", err)
	}

	lines := strings.Split(string(output), "\n")
	var currentInterface string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "Interface") && strings.Contains(line, ":") {
			parts := strings.Split(line, ":")
			if len(parts) > 1 {
				currentInterface = strings.TrimSpace(parts[1])
				currentInterface = strings.Trim(currentInterface, "\"")
			}
		} else if strings.Contains(line, ".") && currentInterface != "" {
			// This looks like an IP address
			if dm.originalDNS[currentInterface] == nil {
				dm.originalDNS[currentInterface] = []string{}
			}
			dm.originalDNS[currentInterface] = append(dm.originalDNS[currentInterface], line)
		}
	}

	return nil
}

func (dm *Manager) setDNSWindows(dnsServer string) error {
	for interfaceName := range dm.originalDNS {
		cmd := exec.Command("netsh", "interface", "ipv4", "set", "dnsservers", interfaceName, "static", dnsServer, "primary")
		if err := cmd.Run(); err != nil {
			log.Warnf("Failed to set DNS for interface %s: %v", interfaceName, err)
		} else {
			log.Debugf("Set DNS for Windows interface %s to %s", interfaceName, dnsServer)
		}
	}
	return nil
}

func (dm *Manager) restoreDNSWindows() error {
	for interfaceName, dnsServers := range dm.originalDNS {
		if len(dnsServers) == 0 {
			// Set to automatic
			cmd := exec.Command("netsh", "interface", "ipv4", "set", "dnsservers", interfaceName, "dhcp")
			if err := cmd.Run(); err != nil {
				log.Warnf("Failed to restore DNS for interface %s: %v", interfaceName, err)
			}
		} else {
			// Set primary DNS
			cmd := exec.Command("netsh", "interface", "ipv4", "set", "dnsservers", interfaceName, "static", dnsServers[0], "primary")
			if err := cmd.Run(); err != nil {
				log.Warnf("Failed to restore primary DNS for interface %s: %v", interfaceName, err)
			}

			// Set secondary DNS servers
			for i, dns := range dnsServers[1:] {
				cmd := exec.Command("netsh", "interface", "ipv4", "add", "dnsservers", interfaceName, dns, fmt.Sprintf("index=%d", i+2))
				if err := cmd.Run(); err != nil {
					log.Warnf("Failed to restore secondary DNS %s for interface %s: %v", dns, interfaceName, err)
				}
			}
		}
	}
	return nil
}

// macOS-specific implementations (keeping the same complex logic from the original)
func (dm *Manager) getCurrentDNSMacOS() error {
	// Get list of network services
	cmd := exec.Command("networksetup", "-listallnetworkservices")
	output, err := cmd.Output()
	if err != nil {
		log.Debugf("Failed to list network services: %v", err)
		return fmt.Errorf("failed to list network services: %v", err)
	}

	log.Debugf("Network services output: %s", string(output))
	lines := strings.Split(string(output), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "*") || strings.Contains(line, "asterisk") {
			log.Debugf("Skipping line: %s", line)
			continue
		}

		// Only process active network services
		if !dm.isNetworkServiceActive(line) {
			log.Debugf("Skipping inactive service: %s", line)
			continue
		}

		log.Debugf("Checking DNS for active service: %s", line)

		// Get DNS servers for this service
		cmd := exec.Command("networksetup", "-getdnsservers", line)
		dnsOutput, err := cmd.Output()
		if err != nil {
			log.Debugf("Failed to get DNS for service %s: %v", line, err)
			continue // Skip services we can't query
		}

		log.Debugf("DNS output for %s: %s", line, string(dnsOutput))
		dnsOutputStr := strings.TrimSpace(string(dnsOutput))

		// Skip if DNS is already set to localhost (indicates previous run didn't clean up)
		if strings.Contains(dnsOutputStr, "127.0.0.1") && !strings.Contains(dnsOutputStr, "aren't any") {
			log.Warnf("Service %s already has localhost DNS - automatically resetting to fix previous run", line)

			// Automatically reset this service to empty/automatic
			resetCmd := exec.Command("sudo", "networksetup", "-setdnsservers", line, "empty")
			if err := resetCmd.Run(); err != nil {
				log.Warnf("Failed to automatically reset DNS for service %s: %v", line, err)
				log.Infof("To fix manually, run: sudo networksetup -setdnsservers '%s' empty", line)
				continue
			} else {
				log.Infof("Successfully reset %s to automatic DNS", line)

				// Now re-query the DNS settings for this service
				cmd := exec.Command("networksetup", "-getdnsservers", line)
				newDnsOutput, err := cmd.Output()
				if err != nil {
					log.Debugf("Failed to re-query DNS for service %s after reset: %v", line, err)
					continue
				}

				// Update dnsOutputStr with the new output
				dnsOutputStr = strings.TrimSpace(string(newDnsOutput))
				log.Debugf("DNS output for %s after reset: %s", line, dnsOutputStr)
			}
		}

		// Check if using DHCP DNS
		if strings.Contains(dnsOutputStr, "aren't any") || strings.Contains(dnsOutputStr, "There aren't any") {
			log.Debugf("Service %s is using DHCP DNS, storing empty config", line)
			dm.originalDNS[line] = []string{} // Empty means DHCP
		} else {
			dnsLines := strings.Split(dnsOutputStr, "\n")
			var dnsServers []string

			for _, dnsLine := range dnsLines {
				dnsLine = strings.TrimSpace(dnsLine)
				if dnsLine != "" {
					dnsServers = append(dnsServers, dnsLine)
				}
			}

			if len(dnsServers) > 0 {
				dm.originalDNS[line] = dnsServers
				log.Debugf("Found DNS for macOS service %s: %v", line, dnsServers)
			}
		}
	}

	// If we didn't find any DNS configuration, try to get current resolvers
	if len(dm.originalDNS) == 0 {
		log.Debug("No manageable network services found, trying system resolver")
		if err := dm.getCurrentDNSSystemResolver(); err != nil {
			log.Debugf("Failed to get system resolver: %v", err)
		}
	}

	log.Debugf("Total manageable services with DNS found: %d", len(dm.originalDNS))
	return nil
}

// getCurrentDNSSystemResolver gets DNS from system resolver as fallback
func (dm *Manager) getCurrentDNSSystemResolver() error {
	// Check /etc/resolv.conf
	content, err := os.ReadFile("/etc/resolv.conf")
	if err != nil {
		// Try scutil to get current DNS
		cmd := exec.Command("scutil", "--dns")
		output, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("failed to read resolv.conf and scutil: %v", err)
		}

		// Parse scutil output for nameservers
		lines := strings.Split(string(output), "\n")
		var dnsServers []string
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.Contains(line, "nameserver[") && strings.Contains(line, ":") {
				parts := strings.Split(line, ":")
				if len(parts) >= 2 {
					server := strings.TrimSpace(parts[1])
					if server != "" {
						dnsServers = append(dnsServers, server)
					}
				}
			}
		}

		if len(dnsServers) > 0 {
			dm.originalDNS["system"] = dnsServers
			log.Debugf("Found system DNS via scutil: %v", dnsServers)
		}
		return nil
	}

	// Parse resolv.conf
	lines := strings.Split(string(content), "\n")
	var dnsServers []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "nameserver") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				dnsServers = append(dnsServers, parts[1])
			}
		}
	}

	if len(dnsServers) > 0 {
		dm.originalDNS["resolv.conf"] = dnsServers
		log.Debugf("Found system DNS via resolv.conf: %v", dnsServers)
	}

	return nil
}

func (dm *Manager) setDNSMacOS(dnsServer string) error {
	successCount := 0
	for serviceName := range dm.originalDNS {
		if serviceName == "system" || serviceName == "resolv.conf" {
			continue
		}

		// Check if the service is active/enabled
		if !dm.isNetworkServiceActive(serviceName) {
			log.Debugf("Skipping disabled/inactive service: %s", serviceName)
			continue
		}

		log.Debugf("Setting DNS for active service: %s using: sudo networksetup -setdnsservers '%s' %s", serviceName, serviceName, dnsServer)
		cmd := exec.Command("sudo", "networksetup", "-setdnsservers", serviceName, dnsServer)
		output, err := cmd.CombinedOutput()
		if err != nil {
			log.Warnf("Failed to set DNS for service %s: %v (output: %s)", serviceName, err, string(output))
		} else {
			log.Debugf("Successfully set DNS for macOS service %s to %s", serviceName, dnsServer)
			successCount++
		}
	}

	if successCount == 0 {
		return fmt.Errorf("failed to configure DNS on any network interface")
	}

	log.Debugf("Successfully configured DNS on %d network services", successCount)
	return nil
}

// isNetworkServiceActive checks if a network service is active
func (dm *Manager) isNetworkServiceActive(serviceName string) bool {
	// Check if the interface has a valid IP address (indicating it's active)
	cmd := exec.Command("networksetup", "-getinfo", serviceName)
	output, err := cmd.Output()
	if err != nil {
		log.Debugf("Cannot get info for service %s: %v", serviceName, err)
		return false
	}

	outputStr := string(output)

	// If it says "not connected" or similar, skip it
	if strings.Contains(outputStr, "not connected") ||
		strings.Contains(outputStr, "disabled") ||
		strings.Contains(outputStr, "inactive") {
		return false
	}

	// Check if it has an IP address assigned
	if strings.Contains(outputStr, "IP address:") {
		lines := strings.Split(outputStr, "\n")
		for _, line := range lines {
			if strings.Contains(line, "IP address:") && !strings.Contains(line, "none") {
				return true
			}
		}
	}

	// For services like Tailscale that might not show standard IP info,
	// try a different approach - check if we can actually query DNS settings
	cmd = exec.Command("networksetup", "-getdnsservers", serviceName)
	_, err = cmd.Output()
	return err == nil
}

func (dm *Manager) restoreDNSMacOS() error {
	for serviceName, dnsServers := range dm.originalDNS {
		if serviceName == "system" || serviceName == "resolv.conf" {
			// Skip these as they're fallback methods
			continue
		}

		if len(dnsServers) == 0 {
			// Service was originally using DHCP, reset to empty/automatic
			log.Debugf("Restoring %s to automatic DNS using: sudo networksetup -setdnsservers '%s' empty", serviceName, serviceName)
			cmd := exec.Command("sudo", "networksetup", "-setdnsservers", serviceName, "empty")
			if err := cmd.Run(); err != nil {
				log.Warnf("Failed to restore DNS for service %s to automatic: %v", serviceName, err)
			} else {
				log.Debugf("Successfully restored %s to automatic DNS", serviceName)
			}
		} else {
			// Service had specific DNS servers, restore them
			log.Debugf("Restoring %s to specific DNS servers: %v", serviceName, dnsServers)
			args := append([]string{"networksetup", "-setdnsservers", serviceName}, dnsServers...)
			cmd := exec.Command("sudo", args...)
			if err := cmd.Run(); err != nil {
				log.Warnf("Failed to restore DNS servers for service %s: %v", serviceName, err)
			} else {
				log.Debugf("Successfully restored %s to DNS servers: %v", serviceName, dnsServers)
			}
		}
	}
	return nil
}

// GetOriginalDNS returns the original DNS settings
func (dm *Manager) GetOriginalDNS() map[string][]string {
	return dm.originalDNS
}

// SetupSignalHandlers sets up signal handlers for graceful cleanup
func (dm *Manager) SetupSignalHandlers() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		log.Info("Received interrupt signal, cleaning up...")
		dm.RestoreOriginalDNS()
		os.Exit(0)
	}()
}

// ResetAllDNSToAuto resets all network services to automatic (DHCP) DNS
// This is useful when a previous run didn't clean up properly
func (dm *Manager) ResetAllDNSToAuto() error {
	log.Info("Resetting all network services to automatic DNS...")

	// Get list of network services
	cmd := exec.Command("networksetup", "-listallnetworkservices")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to list network services: %v", err)
	}

	lines := strings.Split(string(output), "\n")
	successCount := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "*") || strings.Contains(line, "asterisk") {
			continue
		}

		log.Debugf("Resetting %s to automatic DNS using: sudo networksetup -setdnsservers '%s' empty", line, line)
		cmd := exec.Command("sudo", "networksetup", "-setdnsservers", line, "empty")
		if err := cmd.Run(); err != nil {
			log.Warnf("Failed to reset DNS for service %s: %v", line, err)
		} else {
			log.Debugf("Successfully reset %s to automatic DNS", line)
			successCount++
		}
	}

	if successCount > 0 {
		log.Infof("Successfully reset %d network services to automatic DNS", successCount)
		return nil
	}

	return fmt.Errorf("failed to reset any network services")
}
