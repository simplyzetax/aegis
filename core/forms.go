package core

import (
	"log"
	"strings"

	"github.com/charmbracelet/huh"
)

var SelectedCert string

const createNew = "CREATE_NEW_CERT"

func CertSelectorForm() {
	certNames, err := ListCerts()
	if err != nil {
		log.Fatalf("failed to list certs: %v", err)
	}

	var options []huh.Option[string]
	for _, cert := range certNames {
		options = append(options, huh.NewOption(cert, cert))
	}

	options = append(options, huh.NewOption("✨ Create new certificate...", createNew))

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Pick a certificate.").
				Options(options...).
				Value(&SelectedCert),
		),
	)

	if err := form.Run(); err != nil {
		log.Fatal(err)
	}

	if SelectedCert == createNew {
		var newCertHost string
		inputForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Enter domain for new certificate").
					Value(&newCertHost),
			),
		)
		if err := inputForm.Run(); err != nil {
			log.Fatal(err)
		}

		if err := GenerateCerts(newCertHost); err != nil {
			log.Fatalf("failed to generate new cert: %v", err)
		}

		SelectedCert = strings.ReplaceAll(newCertHost, "*", "_")
	}
}
