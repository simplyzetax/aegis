# Aegis

Aegis is a local DNS server and HTTPS proxy designed to redirect game traffic to custom backends. Perfect for connecting to private Fortnite servers or other game backends that require DNS redirection and SSL termination.

## Features

### 🌐 **DNS Server**

- **Domain Redirection:** Redirect specific domains (like `*.ol.epicgames.com`) to your custom backend ip
- **Wildcard Support:** Use `*.domain.com` patterns to catch all subdomains
- **Upstream Forwarding:** All non-redirected queries go to your regular DNS (Cloudflare by default)
- **System Integration:** Automatically configure your system to use Aegis as DNS server

### 🔒 **HTTPS Proxy**

- **SSL Termination:** Handle HTTPS connections and forward to your HTTP(s) backend
- **Certificate Management:** Generate and manage SSL certificates automatically
- **Custom Headers:** Configure any headers to be injected into backend requests

### ⚙️ **Easy Configuration**

- **JSON Configuration:** Simple config file for all settings
- **Interactive Setup:** Guided certificate selection on startup
- **Hot Reload:** Update DNS redirects without restart
- **Cross Platform:** Works on Windows, macOS, and Linux

## Quick Start

1. **Download and build:**

   ```bash
   git clone https://github.com/simplyzetax/aegis.git
   cd aegis
   go mod tidy
   ```

2. **Configure your backend:**
   Edit `config.json` to point to your backend:

   ```json
   {
     "proxy": {
       "upstream_url": "http://localhost:8787"
     }
   }
   ```

3. **Run with admin privileges:**

   ```bash
   # On Windows (run as Administrator)
   go run main.go

   # On macOS/Linux
   sudo go run main.go
   ```

4. **Follow the prompts** to create SSL certificates for your domains

Aegis will automatically configure your system DNS and start redirecting traffic!

## Configuration

The `config.json` file controls all Aegis behavior:

```json
{
  "dns": {
    "redirects": [
      {
        "domain": "*.ol.epicgames.com",
        "target": "127.0.0.1",
        "description": "Epic Games Online Services",
        "enabled": true
      }
    ],
    "upstream_dns": "1.1.1.1:53",
    "port": "53",
    "auto_manage_system": true
  },
  "proxy": {
    "upstream_url": "http://localhost:8787",
    "port": "443",
    "headers": {
      "X-Telemachus-Identifier": "",
      "X-Custom-Header": "custom-value",
      "X-Game-Backend": "aegis-proxy"
    }
  },
  "identifier": "dev",
  "log_level": "info"
}
```

### DNS Redirects

- **domain:** The domain pattern to redirect (supports wildcards with `*`)
- **target:** IP address to redirect to (usually `127.0.0.1`)
- **enabled:** Toggle redirects on/off without deleting them
- **description:** Human-readable description

### Proxy Settings

- **upstream_url:** Your backend HTTP server URL
- **port:** HTTPS port to listen on (usually 443)
- **headers:** Custom headers to inject into all requests to your backend
  - Add any custom headers your backend needs

### Other Settings

- **upstream_dns:** Where to forward non-redirected DNS queries
- **auto_manage_system:** Automatically configure system DNS settings
- **log_level:** `debug`, `info`, `warn`, or `error`

## How It Works

1. **DNS Interception:** Aegis runs a DNS server that catches domain queries
2. **Selective Redirection:** Configured domains get redirected to your local IP
3. **HTTPS Proxy:** Aegis terminates SSL and forwards HTTP to your backend
4. **System Integration:** Your system is configured to use Aegis for DNS

## Requirements

- **Go 1.18+** for building from source
- **Administrator/root privileges** for DNS port binding and system configuration
- **Your game backend** running on HTTP (Aegis handles the HTTPS part)

## Use Cases

- **Custom Fortnite backends** - Redirect Epic Games domains to your server
- **Game server development** - Test local backends with real game clients
- **Local development** - HTTPS proxy for any HTTP service
- **Network testing** - Redirect any domain for testing purposes

## Troubleshooting

**Port 53 already in use?** - Stop other DNS services or change the port in config

**SSL certificate errors?** - Run the interactive setup again to generate new certificates

**DNS not working?** - Make sure Aegis is running with admin privileges

**Can't connect to backend?** - Verify your `upstream_url` is correct and backend is running
