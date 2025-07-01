package proxy

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/proxy"
	"github.com/simplyzetax/aegis/internal/config"
)

// Handler processes incoming HTTP requests and proxies them to the upstream server
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

	// Set custom headers from configuration
	for headerName, headerValue := range config.Config.Proxy.Headers {
		c.Request().Header.Set(headerName, headerValue)
	}

	// Build upstream URL with path and query parameters
	upstreamURL := config.Config.Proxy.UpstreamURL + string(url.Path()) + "?" + string(url.QueryString())

	// Proxy the request to the upstream server
	if err := proxy.Do(c, upstreamURL); err != nil {
		return err
	}
	return nil
}
