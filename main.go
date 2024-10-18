package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

const (
	ansiBold       = "\033[1m"
	ansiGreen      = "\033[32m"
	ansiGrey       = "\033[90m"
	ansiRed        = "\033[31m"
	ansiReset      = "\033[0m"
	cancelAfter    = 100 * time.Millisecond
	expectedArgs   = 3
	maxDepth       = 5
	parentTimeout  = 3 * time.Second
	resolveTimeout = 3 * time.Second
	resolverAddr   = "8.8.8.8"
)

func main() {
	if len(os.Args) != expectedArgs {
		fmt.Println("Usage: dnsv <domain> <query>")
		os.Exit(1)
	}
	domain := os.Args[1]
	queryType := os.Args[2]

	fmt.Println("Starting DNS visualization...")
	visualizeDNSResolution(domain, 1, queryType, parentTimeout)
}

// visualizeDNSResolution handles the DNS query recursively, printing out the path.
func visualizeDNSResolution(domain string, depth int, queryType string, parentTimeout time.Duration) {
	if depth > maxDepth { // Limit recursion depth to prevent infinite loops
		return
	}

	canceled := parentTimeout < cancelAfter // Simulate a cancel timeout for some requests
	result := resolve(domain, resolverAddr, queryType)
	displayResult(resolverAddr, result, depth, canceled)

	// Continue recursion unless canceled or there was an error
	if !canceled && result.Error == nil {
		visualizeDNSResolution(domain, depth+1, "NS", parentTimeout-result.TimeTaken)
	}
}

// resolve performs a DNS resolution for the given domain and returns the result.
func resolve(domain, server, queryType string) DNSResult {
	start := time.Now()
	var ns string
	if server == "" {
		ns = "root"        // Root server
		server = "8.8.8.8" // Default to Google's DNS server
	} else {
		ns = server
	}

	ctx, cancel := context.WithTimeout(context.Background(), resolveTimeout)
	defer cancel()

	r := net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ /* address */ string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: time.Second,
			}
			return d.DialContext(ctx, network, server+":53")
		},
	}

	var records []string
	var err error
	switch queryType {
	case "NS":
		nsRecords, lookupErr := r.LookupNS(ctx, domain)
		err = lookupErr
		if err == nil {
			for _, ns := range nsRecords {
				records = append(records, ns.Host)
			}
		}
	default:
		records, err = r.LookupHost(ctx, domain)
	}

	duration := time.Since(start)
	if err != nil {
		return DNSResult{
			Query:       domain,
			Server:      ns,
			QueryType:   queryType,
			TimeTaken:   duration,
			ResponseMsg: "NXDOMAIN",
			Error:       err,
		}
	}

	return DNSResult{
		Query:       domain,
		Server:      ns,
		QueryType:   queryType,
		TimeTaken:   duration,
		ResponseMsg: strings.Join(records, ", "),
		Error:       nil,
	}
}

// DNSResult stores the result of a DNS query.
type DNSResult struct {
	Query       string
	Server      string
	QueryType   string
	TimeTaken   time.Duration
	ResponseMsg string
	Error       error
}

// displayResult prints the DNS result in a structured format.
func displayResult(server string, result DNSResult, depth int, canceled bool) { // nolint:revive // flag-parameter no control flag
	if depth == 1 {
		printHeader("DNS server", server)
	}
	indent := strings.Repeat("│   ", depth-1)
	if depth > 0 {
		fmt.Printf("%s╭─── resolve(%sdomain:%s %q, %squery:%s %q, %sdepth:%s %d)\n", indent, ansiGreen, ansiReset, result.Query, ansiGreen, ansiReset, result.QueryType, ansiGreen, ansiReset, depth)
	}

	status := "OK"
	if canceled {
		status = "CANCELED"
	}

	msg := fmt.Sprintf("%dms:", result.TimeTaken.Milliseconds())
	if result.Error != nil {
		msg += ansiGrey + " # ERROR: " + result.ResponseMsg + ansiReset
	} else {
		msg += ansiGrey + " # " + result.ResponseMsg + ansiReset
	}
	if canceled {
		msg += " == " + status + " =="
	}

	fmt.Printf("%s╰─── %s\n", indent, msg)
}

func printHeader(header, msg string) {
	fmt.Printf("\n%s%s%s:%s %s\n\n", ansiBold, ansiRed, header, ansiReset, msg)
}
