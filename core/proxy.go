package core

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/proxy"
)

const (
	hybridURL = "http://localhost:8787"
)

func Handler(c *fiber.Ctx) error {
	epicURL := c.Request().URI()

	c.Request().Header.Set("X-Epic-URL", epicURL.String())

	if err := proxy.Do(c, hybridURL); err != nil {
		return err
	}
	return nil
}
