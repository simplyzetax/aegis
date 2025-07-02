package ssl

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/log"
)

// GenerateCerts creates a self-signed SSL certificate and key compatible with Chrome.
// It includes a Subject Alternative Name (SAN) which is required by modern browsers.
// The certificate and key are saved to cert.pem and key.pem in the appropriate directory.
func GenerateCerts(host string) error {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}

	// Valid for 10 years
	notAfter := time.Now().Add(10 * 365 * 24 * time.Hour)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return fmt.Errorf("failed to generate serial number: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Aegis Development"},
		},
		NotBefore: time.Now(),
		NotAfter:  notAfter,

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  false,
	}

	if ip := net.ParseIP(host); ip != nil {
		template.IPAddresses = append(template.IPAddresses, ip)
	} else {
		template.DNSNames = append(template.DNSNames, host)
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return fmt.Errorf("failed to create certificate: %w", err)
	}

	// Replace wildcard for filesystem friendliness
	safeHost := strings.ReplaceAll(host, "*", "_")
	certDir := filepath.Join("certs", safeHost)

	if err := os.MkdirAll(certDir, 0755); err != nil {
		return fmt.Errorf("failed to create certs directory: %w", err)
	}

	certPath := filepath.Join(certDir, "cert.pem")
	keyPath := filepath.Join(certDir, "key.pem")

	certOut, err := os.Create(certPath)
	if err != nil {
		return fmt.Errorf("failed to open %s for writing: %w", certPath, err)
	}
	defer certOut.Close()
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return fmt.Errorf("failed to write data to %s: %w", certPath, err)
	}
	log.Printf("Certificate written to %s\n", certPath)

	keyOut, err := os.Create(keyPath)
	if err != nil {
		return fmt.Errorf("failed to open %s for writing: %w", keyPath, err)
	}
	defer keyOut.Close()

	privBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return fmt.Errorf("unable to marshal ECDSA private key: %w", err)
	}

	if err := pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes}); err != nil {
		return fmt.Errorf("failed to write data to %s: %w", keyPath, err)
	}
	log.Printf("Private key written to %s\n", keyPath)

	return nil
}

// LoadCert loads a certificate and key pair from the specified certificate name
func LoadCert(name string) tls.Certificate {
	certPath := filepath.Join("certs", name, "cert.pem")
	keyPath := filepath.Join("certs", name, "key.pem")
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		log.Fatalf("failed to load cert %s: %v", name, err)
	}
	return cert
}

// ListCerts returns a list of available certificate names
func ListCerts() ([]string, error) {
	certDir := "certs"
	files, err := os.ReadDir(certDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read certs directory: %w", err)
	}

	var certNames []string
	for _, file := range files {
		if file.IsDir() {
			certNames = append(certNames, file.Name())
		}
	}

	return certNames, nil
}

// ValidateCert checks if a certificate exists and is valid
func ValidateCert(name string) error {
	certPath := filepath.Join("certs", name, "cert.pem")
	keyPath := filepath.Join("certs", name, "key.pem")

	// Check if files exist
	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		return fmt.Errorf("certificate file not found: %s", certPath)
	}
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		return fmt.Errorf("key file not found: %s", keyPath)
	}

	// Try to load the certificate to validate it
	_, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return fmt.Errorf("invalid certificate or key: %v", err)
	}

	return nil
}

// GetCertInfo returns information about a certificate
func GetCertInfo(name string) (map[string]interface{}, error) {
	certPath := filepath.Join("certs", name, "cert.pem")

	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read certificate: %v", err)
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %v", err)
	}

	return map[string]interface{}{
		"subject":      cert.Subject.String(),
		"issuer":       cert.Issuer.String(),
		"not_before":   cert.NotBefore,
		"not_after":    cert.NotAfter,
		"dns_names":    cert.DNSNames,
		"ip_addresses": cert.IPAddresses,
		"serial":       cert.SerialNumber.String(),
		"is_ca":        cert.IsCA,
	}, nil
}
