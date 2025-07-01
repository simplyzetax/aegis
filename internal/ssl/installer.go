package ssl

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/log"
)

// CertInstaller handles installing certificates to the system trust store
type CertInstaller struct {
	platform string
}

// NewCertInstaller creates a new certificate installer
func NewCertInstaller() *CertInstaller {
	return &CertInstaller{
		platform: runtime.GOOS,
	}
}

// InstallCertificate installs a certificate to the system trust store
func (ci *CertInstaller) InstallCertificate(certName string) error {
	certPath := filepath.Join("certs", certName, "cert.pem")

	// Verify certificate exists
	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		return fmt.Errorf("certificate file not found: %s", certPath)
	}

	switch ci.platform {
	case "windows":
		return ci.installCertificateWindows(certPath, certName)
	case "darwin":
		return ci.installCertificateMacOS(certPath, certName)
	default:
		return fmt.Errorf("certificate installation not supported on %s", ci.platform)
	}
}

// UninstallCertificate removes a certificate from the system trust store
func (ci *CertInstaller) UninstallCertificate(certName string) error {
	switch ci.platform {
	case "windows":
		return ci.uninstallCertificateWindows(certName)
	case "darwin":
		return ci.uninstallCertificateMacOS(certName)
	default:
		return fmt.Errorf("certificate uninstallation not supported on %s", ci.platform)
	}
}

// IsInstalled checks if a certificate is installed in the system trust store
func (ci *CertInstaller) IsInstalled(certName string) (bool, error) {
	switch ci.platform {
	case "windows":
		return ci.isInstalledWindows(certName)
	case "darwin":
		return ci.isInstalledMacOS(certName)
	default:
		return false, fmt.Errorf("certificate check not supported on %s", ci.platform)
	}
}

// Windows implementation
func (ci *CertInstaller) installCertificateWindows(certPath, certName string) error {
	log.Infof("Installing certificate %s to Windows certificate store...", certName)

	// Convert relative path to absolute path for PowerShell
	absPath, err := filepath.Abs(certPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %v", err)
	}

	// PowerShell command to install certificate to Trusted Root Certification Authorities
	psScript := fmt.Sprintf(`
		$cert = New-Object System.Security.Cryptography.X509Certificates.X509Certificate2('%s')
		$store = New-Object System.Security.Cryptography.X509Certificates.X509Store('Root', 'LocalMachine')
		$store.Open('ReadWrite')
		$store.Add($cert)
		$store.Close()
		Write-Host "Certificate installed successfully"
	`, strings.ReplaceAll(absPath, `\`, `\\`))

	cmd := exec.Command("powershell", "-ExecutionPolicy", "Bypass", "-Command", psScript)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to install certificate: %v\nOutput: %s", err, string(output))
	}

	log.Infof("Certificate %s installed successfully to Windows certificate store", certName)
	return nil
}

func (ci *CertInstaller) uninstallCertificateWindows(certName string) error {
	log.Infof("Removing certificate %s from Windows certificate store...", certName)

	// PowerShell script to remove certificate by subject name
	psScript := fmt.Sprintf(`
		$store = New-Object System.Security.Cryptography.X509Certificates.X509Store('Root', 'LocalMachine')
		$store.Open('ReadWrite')
		$certs = $store.Certificates | Where-Object {$_.Subject -like "*Aegis Development*"}
		foreach ($cert in $certs) {
			$store.Remove($cert)
			Write-Host "Removed certificate: $($cert.Subject)"
		}
		$store.Close()
	`)

	cmd := exec.Command("powershell", "-ExecutionPolicy", "Bypass", "-Command", psScript)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to uninstall certificate: %v\nOutput: %s", err, string(output))
	}

	log.Infof("Certificate %s removed from Windows certificate store", certName)
	return nil
}

func (ci *CertInstaller) isInstalledWindows(certName string) (bool, error) {
	psScript := `
		$store = New-Object System.Security.Cryptography.X509Certificates.X509Store('Root', 'LocalMachine')
		$store.Open('ReadOnly')
		$certs = $store.Certificates | Where-Object {$_.Subject -like "*Aegis Development*"}
		$store.Close()
		if ($certs.Count -gt 0) { Write-Host "true" } else { Write-Host "false" }
	`

	cmd := exec.Command("powershell", "-ExecutionPolicy", "Bypass", "-Command", psScript)
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to check certificate status: %v", err)
	}

	return strings.TrimSpace(string(output)) == "true", nil
}

// macOS implementation
func (ci *CertInstaller) installCertificateMacOS(certPath, certName string) error {
	log.Infof("Installing certificate %s to macOS Keychain...", certName)

	// Convert relative path to absolute path
	absPath, err := filepath.Abs(certPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %v", err)
	}

	// Install certificate to System keychain and mark as trusted for SSL
	cmd := exec.Command("sudo", "security", "add-trusted-cert",
		"-d", "-r", "trustRoot",
		"-k", "/Library/Keychains/System.keychain",
		absPath)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to install certificate: %v\nOutput: %s", err, string(output))
	}

	log.Infof("Certificate %s installed successfully to macOS Keychain", certName)
	return nil
}

func (ci *CertInstaller) uninstallCertificateMacOS(certName string) error {
	log.Infof("Removing certificate %s from macOS Keychain...", certName)

	// Method 1: Try to delete by common name
	cmd := exec.Command("sudo", "security", "delete-certificate",
		"-c", "Aegis Development",
		"/Library/Keychains/System.keychain")

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Debugf("Method 1 output: %s", string(output))
	}

	// Method 2: Find and delete by hash (more reliable)
	// First find all certificates by our organization
	findCmd := exec.Command("bash", "-c",
		`security find-certificate -a -c "Aegis Development" -Z /Library/Keychains/System.keychain | grep "SHA-1 hash:" | cut -d' ' -f3`)

	hashOutput, err := findCmd.Output()
	if err == nil && len(hashOutput) > 0 {
		hashes := strings.Split(strings.TrimSpace(string(hashOutput)), "\n")
		for _, hash := range hashes {
			if hash != "" {
				deleteCmd := exec.Command("sudo", "security", "delete-certificate",
					"-Z", hash, "/Library/Keychains/System.keychain")
				deleteOutput, deleteErr := deleteCmd.CombinedOutput()
				if deleteErr != nil {
					log.Debugf("Failed to delete certificate with hash %s: %v, output: %s", hash, deleteErr, string(deleteOutput))
				} else {
					log.Debugf("Successfully deleted certificate with hash %s", hash)
				}
			}
		}
	}

	log.Infof("Certificate %s removal attempted from macOS Keychain", certName)
	return nil
}

func (ci *CertInstaller) isInstalledMacOS(certName string) (bool, error) {
	// Try multiple methods to find the certificate

	// Method 1: Search by common name (organization)
	cmd := exec.Command("security", "find-certificate",
		"-c", "Aegis Development",
		"/Library/Keychains/System.keychain")

	if err := cmd.Run(); err == nil {
		return true, nil
	}

	// Method 2: Search by subject using grep (more reliable)
	cmd = exec.Command("security", "dump-keychain", "/Library/Keychains/System.keychain")
	output, err := cmd.Output()
	if err == nil {
		// Check if the output contains our organization
		if strings.Contains(string(output), "Aegis Development") {
			return true, nil
		}
	}

	// Method 3: Use security find-certificate with -a (all) and grep
	cmd = exec.Command("bash", "-c",
		`security find-certificate -a /Library/Keychains/System.keychain | grep -i "aegis development" >/dev/null 2>&1`)

	if err := cmd.Run(); err == nil {
		return true, nil
	}

	// Method 4: Check if any certificate with our subject exists
	cmd = exec.Command("bash", "-c",
		`security find-certificate -p -c "Aegis Development" /Library/Keychains/System.keychain >/dev/null 2>&1`)

	return cmd.Run() == nil, nil
}

// InstallCertificateToSystem is a convenience function for installing certificates
func InstallCertificateToSystem(certName string) error {
	installer := NewCertInstaller()
	return installer.InstallCertificate(certName)
}

// UninstallCertificateFromSystem is a convenience function for uninstalling certificates
func UninstallCertificateFromSystem(certName string) error {
	installer := NewCertInstaller()
	return installer.UninstallCertificate(certName)
}

// IsCertificateInstalled checks if a certificate is installed in the system
func IsCertificateInstalled(certName string) (bool, error) {
	installer := NewCertInstaller()
	return installer.IsInstalled(certName)
}

// CleanupAllInstalledCerts removes all Aegis certificates from the system
func CleanupAllInstalledCerts() error {
	installer := NewCertInstaller()

	// Get list of all certificates
	certs, err := ListCerts()
	if err != nil {
		return fmt.Errorf("failed to list certificates: %v", err)
	}

	var errors []string
	for _, cert := range certs {
		if err := installer.UninstallCertificate(cert); err != nil {
			errors = append(errors, fmt.Sprintf("failed to uninstall %s: %v", cert, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors during cleanup: %s", strings.Join(errors, "; "))
	}

	log.Info("All Aegis certificates cleaned up from system")
	return nil
}

// GetSupportedPlatforms returns platforms that support certificate installation
func GetSupportedPlatforms() []string {
	return []string{"windows", "darwin"}
}

// IsPlatformSupported checks if current platform supports certificate installation
func IsPlatformSupported() bool {
	supported := GetSupportedPlatforms()
	for _, platform := range supported {
		if platform == runtime.GOOS {
			return true
		}
	}
	return false
}
