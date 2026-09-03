// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package events

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"time"
)

// defaultBlockedCIDRs are the ranges a webhook must not target unless the
// operator overrides them. They cover loopback, private, link-local (which
// includes the 169.254.169.254 cloud metadata endpoint), multicast and
// unspecified ranges for both IPv4 and IPv6.
var defaultBlockedCIDRs = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("169.254.0.0/16"),
	netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.168.0.0/16"),
	netip.MustParsePrefix("224.0.0.0/4"),
	netip.MustParsePrefix("::/128"),
	netip.MustParsePrefix("::1/128"),
	netip.MustParsePrefix("fc00::/7"),
	netip.MustParsePrefix("fe80::/10"),
	netip.MustParsePrefix("ff00::/8"),
}

// webhookBlockedNets is the block-list consulted by isBlockedIP. It is
// configured once at startup and read without locking.
var webhookBlockedNets = defaultBlockedCIDRs

// SetWebhookBlockedCIDRs installs the list of CIDR ranges that webhooks may
// not target. A nil list restores the built-in defaults; an empty non-nil list
// blocks nothing. It is meant to be called once at startup, before any webhook
// is validated or delivered.
func SetWebhookBlockedCIDRs(cidrs []netip.Prefix) {
	if cidrs == nil {
		cidrs = defaultBlockedCIDRs
	}
	webhookBlockedNets = cidrs
}

// isBlockedIP reports whether an address must not be reached by a webhook,
// i.e. whether it falls inside one of the configured blocked ranges.
func isBlockedIP(ip net.IP) bool {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return true
	}
	addr = addr.Unmap()
	for _, n := range webhookBlockedNets {
		if n.Contains(addr) {
			return true
		}
	}
	return false
}

// safeDialContext resolves the target host and refuses to connect if any
// resolved address points at an internal range. It then dials the exact
// address it validated so a name that re-resolves to an internal IP (DNS
// rebinding) cannot slip through.
func safeDialContext(
	ctx context.Context, network, addr string,
) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}

	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("no addresses for %q", host)
	}
	for _, ip := range ips {
		if isBlockedIP(ip.IP) {
			return nil, fmt.Errorf("webhook target %s is not allowed", ip.IP)
		}
	}

	dialer := &net.Dialer{Timeout: 10 * time.Second}
	return dialer.DialContext(
		ctx, network, net.JoinHostPort(ips[0].IP.String(), port),
	)
}

// safeWebhookClient is used for all webhook deliveries. Its dialer rejects
// internal addresses on the initial request and on every redirect hop (each
// hop triggers a fresh dial).
var safeWebhookClient = &http.Client{
	Transport: &http.Transport{DialContext: safeDialContext},
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("too many redirects")
		}
		if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
			return fmt.Errorf("disallowed redirect scheme %q", req.URL.Scheme)
		}
		return nil
	},
}

const webhookResolveTimeout = 5 * time.Second

// ValidateWebhookURL checks a webhook URL when it is created or updated so
// operators get immediate feedback instead of a silent failure at delivery
// time. It requires an http(s) scheme, forbids embedded credentials, and
// rejects any address in an internal range. Host names are resolved (with
// a bounded timeout) and every returned address is checked. A name that cannot
// be resolved is not rejected here: it may be transient or not yet up, and the
// dialer re-validates at delivery time (which also guards against DNS
// rebinding).
func ValidateWebhookURL(ctx context.Context, rawurl string) error {
	u, err := url.Parse(rawurl)
	if err != nil {
		return fmt.Errorf("invalid URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("URL scheme must be http or https")
	}
	if u.User != nil {
		return fmt.Errorf("URL must not contain credentials")
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("URL must contain a host")
	}
	if ip := net.ParseIP(host); ip != nil {
		if isBlockedIP(ip) {
			return fmt.Errorf("URL points to a disallowed address")
		}
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, webhookResolveTimeout)
	defer cancel()
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil
	}
	for _, ip := range ips {
		if isBlockedIP(ip.IP) {
			return fmt.Errorf("URL resolves to a disallowed address %s", ip.IP)
		}
	}
	return nil
}
