# Aegis

Aegis is a simple, local HTTPS proxy written in Go. It's designed to expose a local webserver running on HTTP to the local network via HTTPS, handling SSL certificate generation and management on the fly.

## Features

- **Local HTTPS Proxy:** Exposes a local HTTP server over HTTPS.
- **Automatic Certificate Management:** Can generate self-signed SSL certificates for any hostname, including wildcard domains.
- **Interactive Certificate Selection:** On startup, it provides an interactive prompt to choose a previously generated certificate or create a new one.
- **Header Injection:** Injects `X-Telemachus-Identifier` and `X-Epic-URL` headers into the proxied requests.

## Prerequisites

- Go 1.18 or higher.

## Getting Started

1.  **Clone the repository:**

    ```sh
    git clone https://github.com/simplyzetax/aegis.git
    cd aegis
    ```

2.  **Install dependencies:**

    ```sh
    go mod tidy
    ```

3.  **Run the application:**
    ```sh
    go run main.go
    ```

When you first run Aegis, it will prompt you to create a new SSL certificate. Enter a hostname (e.g., `localhost`, `*.example.com`) to generate a certificate. The certificate and key will be stored in the `certs/` directory.

On subsequent runs, you can select the previously created certificate or create a new one.

Aegis will then start an HTTPS server on port `443`.

## Configuration

Aegis proxies all incoming requests to the `upstream_url` defined in config.jsob
