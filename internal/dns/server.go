package dns

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/miekg/dns"
	"github.com/simplyzetax/aegis/internal/config"
)

// Server represents our custom DNS server
type Server struct {
	udpServer         *dns.Server
	tcpServer         *dns.Server
	upstreamDNS       string
	redirects         map[string]string // domain pattern -> target IP
	redirectsWildcard map[string]string // wildcard patterns
}

// NewServer creates a new DNS server instance
func NewServer() *Server {
	server := &Server{
		upstreamDNS:       config.Config.DNS.UpstreamDNS,
		redirects:         make(map[string]string),
		redirectsWildcard: make(map[string]string),
	}

	server.updateRedirects()
	return server
}

// updateRedirects refreshes the redirect maps from configuration
func (s *Server) updateRedirects() {
	s.redirects = make(map[string]string)
	s.redirectsWildcard = make(map[string]string)

	for _, redirect := range config.GetEnabledRedirects() {
		// Normalize domain pattern
		domain := strings.ToLower(redirect.Domain)
		if !strings.HasSuffix(domain, ".") {
			domain += "."
		}

		if strings.HasPrefix(domain, "*.") {
			// Wildcard pattern
			pattern := domain[2:] // Remove "*."
			s.redirectsWildcard[pattern] = redirect.Target
			log.Debugf("Added wildcard redirect: *.%s -> %s", pattern, redirect.Target)
		} else {
			// Exact domain
			s.redirects[domain] = redirect.Target
			log.Debugf("Added exact redirect: %s -> %s", domain, redirect.Target)
		}
	}
}

// handleDNSRequest processes incoming DNS requests
func (s *Server) handleDNSRequest(w dns.ResponseWriter, r *dns.Msg) {
	// Create response message
	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative = true
	m.RecursionAvailable = true

	// Process each question in the request
	for _, q := range r.Question {
		queryName := strings.ToLower(q.Name)

		// Check if this query matches any of our redirects
		targetIP, shouldRedirect := s.shouldRedirectQuery(queryName)

		if shouldRedirect {
			log.Debugf("DNS Query (redirecting): %s %s -> %s", q.Name, dns.TypeToString[q.Qtype], targetIP)
			s.handleRedirectQuery(m, q, targetIP)
		} else {
			// Forward to upstream DNS
			s.forwardToUpstream(m, r, w)
			return
		}
	}

	// Send the response
	if err := w.WriteMsg(m); err != nil {
		log.Errorf("Failed to write DNS response: %v", err)
	}
}

// shouldRedirectQuery checks if a query should be redirected
func (s *Server) shouldRedirectQuery(queryName string) (string, bool) {
	queryName = strings.ToLower(queryName)
	if !strings.HasSuffix(queryName, ".") {
		queryName += "."
	}

	// Check exact matches first
	if targetIP, exists := s.redirects[queryName]; exists {
		return targetIP, true
	}

	// Check wildcard patterns
	for pattern, targetIP := range s.redirectsWildcard {
		if strings.HasSuffix(queryName, pattern) {
			return targetIP, true
		}
	}

	return "", false
}

// handleRedirectQuery handles queries that should be redirected
func (s *Server) handleRedirectQuery(m *dns.Msg, q dns.Question, targetIP string) {
	log.Infof("Redirecting domain: %s -> %s", q.Name, targetIP)

	switch q.Qtype {
	case dns.TypeA:
		// Return A record pointing to target IP
		rr := &dns.A{
			Hdr: dns.RR_Header{
				Name:   q.Name,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    300, // 5 minutes TTL
			},
			A: net.ParseIP(targetIP),
		}
		m.Answer = append(m.Answer, rr)

	case dns.TypeAAAA:
		// For IPv6, try to parse the target as IPv6, fallback to IPv4-mapped
		var ipv6 net.IP
		if ip := net.ParseIP(targetIP); ip != nil {
			if ip.To4() != nil {
				// IPv4 address, map to IPv6
				if targetIP == "127.0.0.1" {
					ipv6 = net.ParseIP("::1")
				} else {
					// Map IPv4 to IPv6
					ipv6 = ip.To16()
				}
			} else {
				// Already IPv6
				ipv6 = ip
			}
		} else {
			// Invalid IP, default to ::1
			ipv6 = net.ParseIP("::1")
		}

		rr := &dns.AAAA{
			Hdr: dns.RR_Header{
				Name:   q.Name,
				Rrtype: dns.TypeAAAA,
				Class:  dns.ClassINET,
				Ttl:    300, // 5 minutes TTL
			},
			AAAA: ipv6,
		}
		m.Answer = append(m.Answer, rr)

	case dns.TypeCNAME:
		// For CNAME queries, return an A record instead
		rr := &dns.A{
			Hdr: dns.RR_Header{
				Name:   q.Name,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    300,
			},
			A: net.ParseIP(targetIP),
		}
		m.Answer = append(m.Answer, rr)

	default:
		// For other record types, return NXDOMAIN
		m.Rcode = dns.RcodeNameError
	}
}

// forwardToUpstream forwards DNS queries to upstream DNS servers
func (s *Server) forwardToUpstream(m *dns.Msg, originalReq *dns.Msg, w dns.ResponseWriter) {
	// Create a new DNS client for forwarding
	client := new(dns.Client)
	client.Timeout = 5 * time.Second

	// Forward the original request to upstream DNS
	resp, _, err := client.Exchange(originalReq, s.upstreamDNS)
	if err != nil {
		log.Errorf("Failed to query upstream DNS %s: %v", s.upstreamDNS, err)
		// Return SERVFAIL if we fail to query upstream
		m.Rcode = dns.RcodeServerFailure
		if err := w.WriteMsg(m); err != nil {
			log.Errorf("Failed to write DNS error response: %v", err)
		}
		return
	}

	// Forward the response from upstream
	if err := w.WriteMsg(resp); err != nil {
		log.Errorf("Failed to write DNS response from upstream: %v", err)
	}

	// Log forwarded queries for domains we care about
	for _, q := range originalReq.Question {
		if s.isInterestingDomain(q.Name) {
			log.Debugf("Forwarded to %s: %s %s", s.upstreamDNS, q.Name, dns.TypeToString[q.Qtype])
		}
	}
}

// isInterestingDomain checks if we should log this domain
func (s *Server) isInterestingDomain(domain string) bool {
	domain = strings.ToLower(domain)
	interestingDomains := []string{
		"epicgames.com",
		"unrealengine.com",
		"fortnite.com",
	}

	for _, interesting := range interestingDomains {
		if strings.Contains(domain, interesting) {
			return true
		}
	}
	return false
}

// Start starts the DNS server on the specified address
func (s *Server) Start(address string) error {
	// Update redirects before starting
	s.updateRedirects()

	// Test if we can bind to the ports
	if err := s.testPortAvailability(address); err != nil {
		return err
	}

	// Create UDP server
	s.udpServer = &dns.Server{
		Addr:    address,
		Net:     "udp",
		Handler: dns.HandlerFunc(s.handleDNSRequest),
	}

	// Create TCP server
	s.tcpServer = &dns.Server{
		Addr:    address,
		Net:     "tcp",
		Handler: dns.HandlerFunc(s.handleDNSRequest),
	}

	log.Infof("Starting DNS server on %s (UDP/TCP)", address)

	// Start UDP server in a goroutine
	go func() {
		if err := s.udpServer.ListenAndServe(); err != nil {
			log.Errorf("Failed to start DNS UDP server: %v", err)
		}
	}()

	// Start TCP server in a goroutine
	go func() {
		if err := s.tcpServer.ListenAndServe(); err != nil {
			log.Errorf("Failed to start DNS TCP server: %v", err)
		}
	}()

	// Give the servers a moment to start
	time.Sleep(100 * time.Millisecond)

	log.Infof("DNS server started successfully with %d active redirects", len(config.GetEnabledRedirects()))
	return nil
}

// testPortAvailability tests if the UDP and TCP ports are available
func (s *Server) testPortAvailability(address string) error {
	// Test UDP port
	listener, err := net.ListenPacket("udp", address)
	if err != nil {
		return fmt.Errorf("failed to bind UDP port %s: %v", address, err)
	}
	listener.Close()

	// Test TCP port
	tcpListener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to bind TCP port %s: %v", address, err)
	}
	tcpListener.Close()

	return nil
}

// Stop stops the DNS server
func (s *Server) Stop() error {
	var udpErr, tcpErr error

	if s.udpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		udpErr = s.udpServer.ShutdownContext(ctx)
	}

	if s.tcpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		tcpErr = s.tcpServer.ShutdownContext(ctx)
	}

	if udpErr != nil {
		return udpErr
	}
	return tcpErr
}

// ReloadRedirects updates the server's redirect configuration
func (s *Server) ReloadRedirects() {
	log.Debug("Reloading DNS redirects from configuration")
	s.updateRedirects()
	log.Infof("Reloaded %d active DNS redirects", len(config.GetEnabledRedirects()))
}

// GetRedirectStatus returns information about current redirects
func (s *Server) GetRedirectStatus() map[string]interface{} {
	return map[string]interface{}{
		"exact_redirects":    s.redirects,
		"wildcard_redirects": s.redirectsWildcard,
		"upstream_dns":       s.upstreamDNS,
		"enabled_count":      len(config.GetEnabledRedirects()),
		"total_count":        len(config.Config.DNS.Redirects),
	}
}
