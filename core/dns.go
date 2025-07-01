package core

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/miekg/dns"
)

// DNSServer represents our custom DNS server
type DNSServer struct {
	server       *dns.Server
	cloudflareIP string
}

// global variable to track DNS server instance
var globalDNSServer *DNSServer

// NewDNSServer creates a new DNS server instance
func NewDNSServer() *DNSServer {
	return &DNSServer{
		cloudflareIP: "1.1.1.1:53", // Cloudflare's primary DNS
	}
}

// handleDNSRequest processes incoming DNS requests
func (d *DNSServer) handleDNSRequest(w dns.ResponseWriter, r *dns.Msg) {
	// Create response message
	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative = true
	m.RecursionAvailable = true

	// Process each question in the request
	for _, q := range r.Question {
		//only log if query contains epicgames.com
		if strings.Contains(q.Name, "epicgames.com") {
			log.Debugf("DNS Query: %s %s", q.Name, dns.TypeToString[q.Qtype])
		}

		// Check if this is a query for *.ol.epicgames.com
		if strings.HasSuffix(strings.ToLower(q.Name), ".ol.epicgames.com.") {
			d.handleEpicGamesQuery(m, q)
		} else {
			// Forward to Cloudflare DNS
			d.forwardToCloudflare(m, r, w)
			return
		}
	}

	// Send the response
	if err := w.WriteMsg(m); err != nil {
		log.Errorf("Failed to write DNS response: %v", err)
	}
}

// handleEpicGamesQuery handles queries for *.ol.epicgames.com domains
func (d *DNSServer) handleEpicGamesQuery(m *dns.Msg, q dns.Question) {
	log.Infof("Intercepting Epic Games domain: %s", q.Name)

	switch q.Qtype {
	case dns.TypeA:
		// Return A record pointing to 127.0.0.1
		rr := &dns.A{
			Hdr: dns.RR_Header{
				Name:   q.Name,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    300, // 5 minutes TTL
			},
			A: net.ParseIP("127.0.0.1"),
		}
		m.Answer = append(m.Answer, rr)

	case dns.TypeAAAA:
		// Return AAAA record pointing to ::1 (IPv6 localhost)
		rr := &dns.AAAA{
			Hdr: dns.RR_Header{
				Name:   q.Name,
				Rrtype: dns.TypeAAAA,
				Class:  dns.ClassINET,
				Ttl:    300, // 5 minutes TTL
			},
			AAAA: net.ParseIP("::1"),
		}
		m.Answer = append(m.Answer, rr)

	case dns.TypeCNAME:
		// For CNAME queries, we'll still redirect to localhost
		// but we need to return an A record instead
		rr := &dns.A{
			Hdr: dns.RR_Header{
				Name:   q.Name,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    300,
			},
			A: net.ParseIP("127.0.0.1"),
		}
		m.Answer = append(m.Answer, rr)

	default:
		// For other record types, return NXDOMAIN
		m.Rcode = dns.RcodeNameError
	}
}

// forwardToCloudflare forwards DNS queries to Cloudflare's DNS servers
func (d *DNSServer) forwardToCloudflare(m *dns.Msg, originalReq *dns.Msg, w dns.ResponseWriter) {
	// create a new DNS client for forwarding
	client := new(dns.Client)
	client.Timeout = 5 * time.Second

	// forward the original request to Cloudflare
	resp, _, err := client.Exchange(originalReq, d.cloudflareIP)
	if err != nil {
		log.Errorf("Failed to query Cloudflare DNS: %v", err)
		// return SERVFAIL if we fail to query Cloudflare
		m.Rcode = dns.RcodeServerFailure
		if err := w.WriteMsg(m); err != nil {
			log.Errorf("Failed to write DNS error response: %v", err)
		}
		return
	}

	// forward the response from Cloudflare
	if err := w.WriteMsg(resp); err != nil {
		log.Errorf("Failed to write DNS response from Cloudflare: %v", err)
	}

	// log the forwarded query
	for _, q := range originalReq.Question {
		if strings.Contains(q.Name, "epicgames.com") {
			log.Debugf("Forwarded to Cloudflare: %s %s", q.Name, dns.TypeToString[q.Qtype])
		}
	}
}

// Start starts the DNS server on the specified address
func (d *DNSServer) Start(address string) error {
	// create DNS server for UDP
	d.server = &dns.Server{
		Addr:    address,
		Net:     "udp",
		Handler: dns.HandlerFunc(d.handleDNSRequest),
	}

	// test if we can bind to the UDP port
	listener, err := net.ListenPacket("udp", address)
	if err != nil {
		return fmt.Errorf("failed to bind UDP port %s: %v", address, err)
	}
	listener.Close()

	// test if we can bind to the TCP port
	tcpListener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to bind TCP port %s: %v", address, err)
	}
	tcpListener.Close()

	log.Infof("Starting DNS server on %s (UDP)", address)

	// start the UDP server in a goroutine
	go func() {
		if err := d.server.ListenAndServe(); err != nil {
			log.Errorf("Failed to start DNS UDP server: %v", err)
		}
	}()

	// create TCP server with its own handler
	tcpServer := &dns.Server{
		Addr:    address,
		Net:     "tcp",
		Handler: dns.HandlerFunc(d.handleDNSRequest),
	}

	log.Infof("Starting DNS server on %s (TCP)", address)

	// start the TCP server in a goroutine
	go func() {
		if err := tcpServer.ListenAndServe(); err != nil {
			log.Errorf("Failed to start DNS TCP server: %v", err)
		}
	}()

	// Give the servers a moment to start
	time.Sleep(100 * time.Millisecond)

	return nil
}

// basically does what it says lol
func (d *DNSServer) Stop() error {
	if d.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return d.server.ShutdownContext(ctx)
	}
	return nil
}

// StartDNSServer is a convenience function to start the DNS server
func StartDNSServer() (string, error) {
	globalDNSServer = NewDNSServer()

	// Try port 53 first since we have admin privileges, then fall back to other ports
	ports := []string{":53", ":8053", ":5353", ":9053", ":10053"}

	var lastErr error
	for _, port := range ports {
		log.Debugf("Trying to start DNS server on port %s", port)
		if err := globalDNSServer.Start(port); err != nil {
			lastErr = err
			log.Debugf("Port %s failed: %v", port, err)
			continue
		}
		log.Infof("DNS server successfully started on port %s", port)
		return port, nil
	}

	return "", lastErr
}

// StopDNSServer stops the global DNS server
func StopDNSServer() error {
	if globalDNSServer != nil {
		return globalDNSServer.Stop()
	}
	return nil
}

// TestDNSServer tests the DNS server functionality
func TestDNSServer(port string) error {
	// Create a test DNS client
	client := new(dns.Client)
	client.Timeout = 2 * time.Second

	// Test Epic Games domain redirection
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn("test.ol.epicgames.com"), dns.TypeA)

	// Use the actual server address
	serverAddr := fmt.Sprintf("127.0.0.1%s", port)
	resp, _, err := client.Exchange(m, serverAddr)
	if err != nil {
		return err
	}

	if len(resp.Answer) == 0 {
		return fmt.Errorf("no DNS answer received")
	}

	// Check if we got the expected localhost response
	if a, ok := resp.Answer[0].(*dns.A); ok {
		if a.A.String() == "127.0.0.1" {
			log.Debug("DNS test successful: Epic Games domain redirected to localhost")
			return nil
		}
	}

	return fmt.Errorf("unexpected DNS response")
}
