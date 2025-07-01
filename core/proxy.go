package core

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/proxy"
	"github.com/spf13/viper"
)

func Handler(c *fiber.Ctx) error {
	url := c.Request().URI()

	// Check if user provided X-Epic-URL header first
	if epicURL := string(c.Request().Header.Peek("X-Epic-URL")); epicURL != "" {
		c.Request().Header.Set("X-Epic-URL", epicURL)
	} else if string(url.Host()) != "localhost" {
		c.Request().Header.Set("X-Epic-URL", url.String())
	} else {
		c.Request().Header.Set("X-Epic-URL", string(c.Request().Header.Peek("X-Epic-URL")))
	}

	c.Request().Header.Set("X-Telemachus-Identifier", Config.Identifier)

	UpstreamURL := viper.GetString("upstream_url")

	UpstreamURL = UpstreamURL + string(url.Path()) + "?" + string(url.QueryString())

	if err := proxy.Do(c, UpstreamURL); err != nil {
		return err
	}
	return nil
}
