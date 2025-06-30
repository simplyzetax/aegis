package main

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/simplyzetax/aegis/core"
)

func main() {
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

	println("Listening on port 443 with cert:", core.SelectedCert)

	app.ListenTLSWithCertificate(":443", core.LoadCert(core.SelectedCert))
}
