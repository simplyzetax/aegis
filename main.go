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

	_, err := core.ListCerts()
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

	app.ListenTLSWithCertificate(":443", core.LoadCert(core.SelectedCert))
}
