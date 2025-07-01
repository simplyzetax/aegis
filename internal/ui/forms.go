package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/log"
	"github.com/simplyzetax/aegis/internal/config"
	"github.com/simplyzetax/aegis/internal/ssl"
)

var SelectedCert string

const createNewCert = "CREATE_NEW_CERT"
const manageRedirects = "MANAGE_REDIRECTS"
const manageCertificates = "MANAGE_CERTIFICATES"

// CertSelectorForm shows the certificate selection interface
func CertSelectorForm(listCerts func() ([]string, error), generateCerts func(string) error) error {
	certNames, err := listCerts()
	if err != nil {
		return fmt.Errorf("failed to list certs: %v", err)
	}

	var options []huh.Option[string]
	for _, cert := range certNames {
		options = append(options, huh.NewOption(cert, cert))
	}

	options = append(options, huh.NewOption("✨ Create new certificate...", createNewCert))
	options = append(options, huh.NewOption("🔧 Manage certificates...", manageCertificates))

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Pick a certificate.").
				Options(options...).
				Value(&SelectedCert),
		),
	)

	if err := form.Run(); err != nil {
		return err
	}

	switch SelectedCert {
	case createNewCert:
		return createNewCertificate(generateCerts)
	case manageCertificates:
		return CertificateManagerForm()
	default:
		return nil // Certificate selected
	}
}

// createNewCertificate handles creating a new certificate
func createNewCertificate(generateCerts func(string) error) error {
	var newCertHost string
	var installToSystem bool

	inputForm := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Enter domain for new certificate").
				Description("e.g., localhost, *.example.com, api.local").
				Value(&newCertHost),
		),
	)
	if err := inputForm.Run(); err != nil {
		return err
	}

	// Ask about system installation if platform is supported
	if ssl.IsPlatformSupported() {
		installForm := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title("Install certificate to system trust store?").
					Description("This will make browsers trust the certificate automatically").
					Value(&installToSystem),
			),
		)
		if err := installForm.Run(); err != nil {
			return err
		}
	}

	// Generate the certificate
	if err := generateCerts(newCertHost); err != nil {
		return fmt.Errorf("failed to generate new cert: %v", err)
	}

	SelectedCert = strings.ReplaceAll(newCertHost, "*", "_")

	// Install to system if requested
	if installToSystem {
		if err := ssl.InstallCertificateToSystem(SelectedCert); err != nil {
			log.Errorf("Failed to install certificate to system: %v", err)
			log.Info("Certificate created but not installed to system trust store")
		} else {
			log.Info("Certificate installed to system trust store successfully!")
		}
	}

	return nil
}

// CertificateManagerForm shows the certificate management interface
func CertificateManagerForm() error {
	for {
		action, err := showCertificateMainMenu()
		if err != nil {
			return err
		}

		switch action {
		case "list":
			showCertificateList()
		case "install":
			if err := installCertificateForm(); err != nil {
				log.Errorf("Failed to install certificate: %v", err)
			}
		case "uninstall":
			if err := uninstallCertificateForm(); err != nil {
				log.Errorf("Failed to uninstall certificate: %v", err)
			}
		case "info":
			if err := showCertificateInfoForm(); err != nil {
				log.Errorf("Failed to show certificate info: %v", err)
			}
		case "cleanup":
			if err := cleanupCertificatesForm(); err != nil {
				log.Errorf("Failed to cleanup certificates: %v", err)
			}
		case "exit":
			return nil
		}
	}
}

// showCertificateMainMenu displays the main menu for certificate management
func showCertificateMainMenu() (string, error) {
	var action string
	var options []huh.Option[string]

	options = append(options,
		huh.NewOption("📋 List certificates", "list"),
		huh.NewOption("ℹ️  Certificate information", "info"),
	)

	// Add platform-specific options
	if ssl.IsPlatformSupported() {
		options = append(options,
			huh.NewOption("📥 Install to system", "install"),
			huh.NewOption("📤 Uninstall from system", "uninstall"),
			huh.NewOption("🧹 Cleanup all system certificates", "cleanup"),
		)
	}

	options = append(options, huh.NewOption("🚪 Back to main menu", "exit"))

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Certificate Management").
				Description("Manage your SSL certificates").
				Options(options...).
				Value(&action),
		),
	)

	return action, form.Run()
}

// showCertificateList displays all available certificates
func showCertificateList() {
	certs, err := ssl.ListCerts()
	if err != nil {
		log.Errorf("Failed to list certificates: %v", err)
		return
	}

	if len(certs) == 0 {
		log.Info("No certificates found")
		return
	}

	log.Info("Available certificates:")
	for i, cert := range certs {
		status := ""
		if ssl.IsPlatformSupported() {
			if installed, err := ssl.IsCertificateInstalled(cert); err == nil && installed {
				status = " 🟢 (System Installed)"
			} else {
				status = " ⚪ (Not System Installed)"
			}
		}
		log.Infof("%d. %s%s", i+1, cert, status)
	}
}

// installCertificateForm handles installing certificates to the system
func installCertificateForm() error {
	if !ssl.IsPlatformSupported() {
		log.Warn("Certificate installation not supported on this platform")
		return nil
	}

	certs, err := ssl.ListCerts()
	if err != nil {
		return fmt.Errorf("failed to list certificates: %v", err)
	}

	if len(certs) == 0 {
		log.Info("No certificates available to install")
		return nil
	}

	var selectedCert string
	var options []huh.Option[string]

	for _, cert := range certs {
		installed, _ := ssl.IsCertificateInstalled(cert)
		status := ""
		if installed {
			status = " (Already Installed)"
		}
		options = append(options, huh.NewOption(cert+status, cert))
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select certificate to install").
				Options(options...).
				Value(&selectedCert),
		),
	)

	if err := form.Run(); err != nil {
		return err
	}

	// Check if already installed
	if installed, err := ssl.IsCertificateInstalled(selectedCert); err == nil && installed {
		log.Warnf("Certificate %s is already installed to system", selectedCert)
		return nil
	}

	// Install the certificate
	if err := ssl.InstallCertificateToSystem(selectedCert); err != nil {
		return fmt.Errorf("failed to install certificate: %v", err)
	}

	log.Infof("Certificate %s installed to system trust store successfully!", selectedCert)
	return nil
}

// uninstallCertificateForm handles uninstalling certificates from the system
func uninstallCertificateForm() error {
	if !ssl.IsPlatformSupported() {
		log.Warn("Certificate uninstallation not supported on this platform")
		return nil
	}

	certs, err := ssl.ListCerts()
	if err != nil {
		return fmt.Errorf("failed to list certificates: %v", err)
	}

	var installedCerts []string
	for _, cert := range certs {
		if installed, err := ssl.IsCertificateInstalled(cert); err == nil && installed {
			installedCerts = append(installedCerts, cert)
		}
	}

	if len(installedCerts) == 0 {
		log.Info("No certificates are currently installed to the system")
		return nil
	}

	var selectedCert string
	var options []huh.Option[string]

	for _, cert := range installedCerts {
		options = append(options, huh.NewOption(cert, cert))
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select certificate to uninstall").
				Options(options...).
				Value(&selectedCert),
		),
	)

	if err := form.Run(); err != nil {
		return err
	}

	// Confirm uninstallation
	var confirm bool
	confirmForm := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(fmt.Sprintf("Uninstall certificate %s from system?", selectedCert)).
				Description("This will remove the certificate from the system trust store").
				Value(&confirm),
		),
	)

	if err := confirmForm.Run(); err != nil {
		return err
	}

	if !confirm {
		return nil
	}

	// Uninstall the certificate
	if err := ssl.UninstallCertificateFromSystem(selectedCert); err != nil {
		return fmt.Errorf("failed to uninstall certificate: %v", err)
	}

	log.Infof("Certificate %s uninstalled from system trust store", selectedCert)
	return nil
}

// showCertificateInfoForm displays detailed information about a certificate
func showCertificateInfoForm() error {
	certs, err := ssl.ListCerts()
	if err != nil {
		return fmt.Errorf("failed to list certificates: %v", err)
	}

	if len(certs) == 0 {
		log.Info("No certificates available")
		return nil
	}

	var selectedCert string
	var options []huh.Option[string]

	for _, cert := range certs {
		options = append(options, huh.NewOption(cert, cert))
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select certificate to view").
				Options(options...).
				Value(&selectedCert),
		),
	)

	if err := form.Run(); err != nil {
		return err
	}

	// Get certificate information
	info, err := ssl.GetCertInfo(selectedCert)
	if err != nil {
		return fmt.Errorf("failed to get certificate info: %v", err)
	}

	// Display certificate information
	log.Infof("📋 Certificate Information for: %s", selectedCert)
	log.Infof("   Subject: %s", info["subject"])
	log.Infof("   Issuer: %s", info["issuer"])
	log.Infof("   Valid From: %s", info["not_before"])
	log.Infof("   Valid Until: %s", info["not_after"])
	log.Infof("   Serial Number: %s", info["serial"])
	log.Infof("   Is CA: %t", info["is_ca"])

	if dnsNames, ok := info["dns_names"].([]string); ok && len(dnsNames) > 0 {
		log.Infof("   DNS Names: %s", strings.Join(dnsNames, ", "))
	}

	// Check system installation status
	if ssl.IsPlatformSupported() {
		if installed, err := ssl.IsCertificateInstalled(selectedCert); err == nil {
			status := "Not Installed"
			if installed {
				status = "Installed"
			}
			log.Infof("   System Trust Store: %s", status)
		}
	}

	return nil
}

// cleanupCertificatesForm handles cleaning up all system-installed certificates
func cleanupCertificatesForm() error {
	if !ssl.IsPlatformSupported() {
		log.Warn("Certificate cleanup not supported on this platform")
		return nil
	}

	var confirm bool
	confirmForm := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Remove ALL Aegis certificates from system trust store?").
				Description("This will uninstall all Aegis certificates from the system. This action cannot be undone.").
				Value(&confirm),
		),
	)

	if err := confirmForm.Run(); err != nil {
		return err
	}

	if !confirm {
		return nil
	}

	// Perform cleanup
	if err := ssl.CleanupAllInstalledCerts(); err != nil {
		return fmt.Errorf("failed to cleanup certificates: %v", err)
	}

	log.Info("All Aegis certificates removed from system trust store")
	return nil
}

// DNSRedirectManagerForm shows the DNS redirect management interface
func DNSRedirectManagerForm() error {
	for {
		action, err := showRedirectMainMenu()
		if err != nil {
			return err
		}

		switch action {
		case "list":
			showRedirectList()
		case "add":
			if err := addRedirectForm(); err != nil {
				log.Errorf("Failed to add redirect: %v", err)
			}
		case "edit":
			if err := editRedirectForm(); err != nil {
				log.Errorf("Failed to edit redirect: %v", err)
			}
		case "toggle":
			if err := toggleRedirectForm(); err != nil {
				log.Errorf("Failed to toggle redirect: %v", err)
			}
		case "remove":
			if err := removeRedirectForm(); err != nil {
				log.Errorf("Failed to remove redirect: %v", err)
			}
		case "exit":
			return nil
		}
	}
}

// showRedirectMainMenu displays the main menu for redirect management
func showRedirectMainMenu() (string, error) {
	var action string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("DNS Redirect Management").
				Description("Manage your DNS redirects").
				Options(
					huh.NewOption("📋 List current redirects", "list"),
					huh.NewOption("➕ Add new redirect", "add"),
					huh.NewOption("✏️  Edit redirect", "edit"),
					huh.NewOption("🔄 Toggle redirect on/off", "toggle"),
					huh.NewOption("🗑️  Remove redirect", "remove"),
					huh.NewOption("🚪 Back to main menu", "exit"),
				).
				Value(&action),
		),
	)

	return action, form.Run()
}

// showRedirectList displays all current redirects
func showRedirectList() {
	redirects := config.Config.DNS.Redirects

	if len(redirects) == 0 {
		log.Info("No DNS redirects configured")
		return
	}

	log.Info("Current DNS redirects:")
	for i, redirect := range redirects {
		status := "✅ Enabled"
		if !redirect.Enabled {
			status = "❌ Disabled"
		}
		log.Infof("%d. %s -> %s (%s) [%s]",
			i+1, redirect.Domain, redirect.Target, redirect.Description, status)
	}
}

// addRedirectForm shows the form to add a new redirect
func addRedirectForm() error {
	var domain, target, description string
	enabled := true

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Domain pattern").
				Description("e.g., *.example.com or specific.domain.com").
				Value(&domain),
			huh.NewInput().
				Title("Target IP").
				Description("IP address to redirect to (usually 127.0.0.1)").
				Value(&target).
				Placeholder("127.0.0.1"),
			huh.NewInput().
				Title("Description").
				Description("Human-readable description for this redirect").
				Value(&description),
			huh.NewConfirm().
				Title("Enable this redirect immediately?").
				Value(&enabled),
		),
	)

	if err := form.Run(); err != nil {
		return err
	}

	if domain == "" || target == "" {
		return fmt.Errorf("domain and target are required")
	}

	if description == "" {
		description = fmt.Sprintf("Redirect %s to %s", domain, target)
	}

	redirect := config.DNSRedirect{
		Domain:      domain,
		Target:      target,
		Description: description,
		Enabled:     enabled,
	}

	if err := config.AddRedirect(redirect); err != nil {
		return err
	}

	log.Infof("Added redirect: %s -> %s", domain, target)
	return nil
}

// editRedirectForm shows the form to edit an existing redirect
func editRedirectForm() error {
	if len(config.Config.DNS.Redirects) == 0 {
		log.Info("No redirects to edit")
		return nil
	}

	// Select which redirect to edit
	var selectedIndex int
	var options []huh.Option[int]

	for i, redirect := range config.Config.DNS.Redirects {
		status := "Enabled"
		if !redirect.Enabled {
			status = "Disabled"
		}
		label := fmt.Sprintf("%s -> %s (%s) [%s]",
			redirect.Domain, redirect.Target, redirect.Description, status)
		options = append(options, huh.NewOption(label, i))
	}

	selectForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[int]().
				Title("Select redirect to edit").
				Options(options...).
				Value(&selectedIndex),
		),
	)

	if err := selectForm.Run(); err != nil {
		return err
	}

	// Edit the selected redirect
	redirect := config.Config.DNS.Redirects[selectedIndex]
	domain := redirect.Domain
	target := redirect.Target
	description := redirect.Description
	enabled := redirect.Enabled

	editForm := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Domain pattern").
				Value(&domain),
			huh.NewInput().
				Title("Target IP").
				Value(&target),
			huh.NewInput().
				Title("Description").
				Value(&description),
			huh.NewConfirm().
				Title("Enabled").
				Value(&enabled),
		),
	)

	if err := editForm.Run(); err != nil {
		return err
	}

	// Update the redirect
	config.Config.DNS.Redirects[selectedIndex] = config.DNSRedirect{
		Domain:      domain,
		Target:      target,
		Description: description,
		Enabled:     enabled,
	}

	if err := config.Save(); err != nil {
		return err
	}

	log.Infof("Updated redirect: %s -> %s", domain, target)
	return nil
}

// toggleRedirectForm shows the form to toggle redirects on/off
func toggleRedirectForm() error {
	if len(config.Config.DNS.Redirects) == 0 {
		log.Info("No redirects to toggle")
		return nil
	}

	var selectedIndex int
	var options []huh.Option[int]

	for i, redirect := range config.Config.DNS.Redirects {
		status := "✅ Enabled"
		if !redirect.Enabled {
			status = "❌ Disabled"
		}
		label := fmt.Sprintf("%s -> %s (%s) [%s]",
			redirect.Domain, redirect.Target, redirect.Description, status)
		options = append(options, huh.NewOption(label, i))
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[int]().
				Title("Select redirect to toggle").
				Options(options...).
				Value(&selectedIndex),
		),
	)

	if err := form.Run(); err != nil {
		return err
	}

	if err := config.ToggleRedirect(selectedIndex); err != nil {
		return err
	}

	redirect := config.Config.DNS.Redirects[selectedIndex]
	status := "enabled"
	if !redirect.Enabled {
		status = "disabled"
	}
	log.Infof("Redirect %s is now %s", redirect.Domain, status)
	return nil
}

// removeRedirectForm shows the form to remove redirects
func removeRedirectForm() error {
	if len(config.Config.DNS.Redirects) == 0 {
		log.Info("No redirects to remove")
		return nil
	}

	var selectedIndex int
	var options []huh.Option[int]

	for i, redirect := range config.Config.DNS.Redirects {
		status := "Enabled"
		if !redirect.Enabled {
			status = "Disabled"
		}
		label := fmt.Sprintf("%s -> %s (%s) [%s]",
			redirect.Domain, redirect.Target, redirect.Description, status)
		options = append(options, huh.NewOption(label, i))
	}

	selectForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[int]().
				Title("Select redirect to remove").
				Options(options...).
				Value(&selectedIndex),
		),
	)

	if err := selectForm.Run(); err != nil {
		return err
	}

	redirect := config.Config.DNS.Redirects[selectedIndex]
	var confirm bool

	confirmForm := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(fmt.Sprintf("Remove redirect %s -> %s?", redirect.Domain, redirect.Target)).
				Description("This action cannot be undone").
				Value(&confirm),
		),
	)

	if err := confirmForm.Run(); err != nil {
		return err
	}

	if !confirm {
		return nil
	}

	if err := config.RemoveRedirect(selectedIndex); err != nil {
		return err
	}

	log.Infof("Removed redirect: %s -> %s", redirect.Domain, redirect.Target)
	return nil
}

// ShowStartupMenu shows the main application startup menu
func ShowStartupMenu() (string, error) {
	var action string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Welcome to Aegis").
				Description("Choose an action to get started").
				Options(
					huh.NewOption("🚀 Start proxy server", "start"),
					huh.NewOption("🌐 Manage DNS redirects", "dns"),
					huh.NewOption("🔒 Select certificate", "cert"),
					huh.NewOption("⚙️  Configuration", "config"),
					huh.NewOption("🚪 Exit", "exit"),
				).
				Value(&action),
		),
	)

	return action, form.Run()
}
