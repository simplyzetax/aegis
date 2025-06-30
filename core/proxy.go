package core

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/proxy"
)

const (
	hybridURL = "http://localhost:8787"
)

var (
	Identifier = ""
)

func Handler(c *fiber.Ctx) error {
	url := c.Request().URI()
	if string(url.Host()) != "localhost" {
		c.Request().Header.Set("X-Epic-URL", url.String())
	} else {
		c.Request().Header.Set("X-Epic-URL", string(c.Request().Header.Peek("X-Epic-URL")))
	}

	c.Request().Header.Set("X-Telemachus-Identifier", Identifier)

	if err := proxy.Do(c, hybridURL); err != nil {
		return err
	}
	return nil
}
