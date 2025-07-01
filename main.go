package main

import (
	"github.com/charmbracelet/log"
	"github.com/gofiber/fiber/v2"
	"github.com/simplyzetax/aegis/core"
)

func main() {

	core.LoadConfig()

	log.Debugf("Platform: %s", core.Platform)
	log.Debugf("IsAdmin: %t", core.IsAdmin())
	log.Debugf("CanEscalate: %t", core.CanEscalate())
	log.Debugf("GetUserPrivilegeInfo: %s", core.GetUserPrivilegeInfo())

	if !core.IsAdmin() {
		log.Warn("Aegis needs to be run as admin to function properly")
		if err := core.EscalatePrivileges(); err != nil {
			log.Fatalf("failed to escalate privileges: %v", err)
		}
	}

	// Create DNS manager and get current DNS settings
	dnsManager := core.NewDNSManager()
	log.Info("Getting current DNS settings...")
	if err := dnsManager.GetCurrentDNS(); err != nil {
		log.Warnf("Failed to get current DNS settings: %v", err)
		log.Info("Continuing without DNS management...")
	} else {
		// Check if we found any manageable interfaces
		if len(dnsManager.GetOriginalDNS()) == 0 {
			log.Warn("No manageable network interfaces found - this might be due to previous DNS not being restored")
			log.Info("Attempting to reset all DNS settings to automatic...")

			if err := dnsManager.ResetAllDNSToAuto(); err != nil {
				log.Warnf("Failed to reset DNS automatically: %v", err)
				log.Info("You may need to manually reset DNS with: sudo networksetup -setdnsservers 'Wi-Fi' empty")
			} else {
				log.Info("DNS reset successful, retrying DNS detection...")
				// Retry getting DNS settings after reset
				if err := dnsManager.GetCurrentDNS(); err != nil {
					log.Warnf("Still failed to get DNS settings after reset: %v", err)
				} else {
					log.Infof("Found %d manageable network services after reset", len(dnsManager.GetOriginalDNS()))
				}
			}
		}

		if len(dnsManager.GetOriginalDNS()) > 0 {
			log.Info("Current DNS settings saved")
			// Set up signal handlers for graceful cleanup
			dnsManager.SetupSignalHandlers()
		} else {
			log.Info("Continuing without DNS management...")
		}
	}

	// Start the DNS server (requires admin privileges)
	log.Debug("Starting DNS server...")
	dnsPort, err := core.StartDNSServer()
	if err != nil {
		log.Fatalf("failed to start DNS server: %v", err)
	}

	// Track whether we modified system DNS settings
	modifiedDNS := false

	// Only configure system DNS if we're not running on the standard port 53
	if dnsPort == ":53" {
		log.Info("DNS server is running on standard port 53 - no system DNS configuration needed")
	} else {
		// Configure system DNS to use our server (if we have original settings)
		if len(dnsManager.GetOriginalDNS()) > 0 {
			log.Infof("Configuring system DNS to use local DNS server on port %s...", dnsPort)
			if err := dnsManager.SetDNSToLocal(dnsPort); err != nil {
				log.Warnf("Failed to configure system DNS: %v", err)
				log.Infof("You may need to manually configure DNS to use 127.0.0.1 (server running on port %s)", dnsPort)
			} else {
				log.Info("System DNS configured successfully")
				modifiedDNS = true
			}
		} else {
			log.Infof("DNS management unavailable - manually configure DNS to use 127.0.0.1 (server running on port %s)", dnsPort)
		}
	}

	// Test DNS functionality
	if err := core.TestDNSServer(dnsPort); err != nil {
		log.Warnf("DNS test failed: %v", err)
	}

	_, err = core.ListCerts()
	if err != nil {
		log.Fatalf("failed to list certs: %v", err)
	}

	core.CertSelectorForm()

	app := fiber.New(fiber.Config{
		BodyLimit:       1024 * 1024 * 1024,
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

	app.All("*", core.Handler)

	log.Infof("Listening on port 443 with cert %s and upstream %s", core.SelectedCert, core.Config.UpstreamURL)
	log.Infof("DNS server is running on port %s", dnsPort)
	log.Info("DNS server is redirecting *.ol.epicgames.com to 127.0.0.1 and forwarding other queries to Cloudflare")

	// Cleanup function to restore DNS when the app exits
	defer func() {
		log.Info("Application shutting down...")
		if modifiedDNS {
			dnsManager.RestoreOriginalDNS()
		}
		core.StopDNSServer()
	}()

	app.ListenTLSWithCertificate(":443", core.LoadCert(core.SelectedCert))
}
