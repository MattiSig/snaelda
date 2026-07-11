package respin

import "net"

// blockedCIDRs are address ranges the re-spin fetcher must never connect to,
// beyond the ranges the stdlib net.IP helpers already classify (loopback,
// link-local, private/ULA, multicast, unspecified). These cover carrier-grade
// NAT, IETF-reserved/documentation ranges, benchmarking space, the NAT64
// well-known prefix, and other non-globally-routable blocks that could front an
// internal service. The cloud-metadata endpoint (169.254.169.254) and the IPv6
// link-local metadata form are already caught by the link-local check.
var blockedCIDRs = func() []*net.IPNet {
	cidrs := []string{
		"0.0.0.0/8",          // "this" network
		"100.64.0.0/10",      // RFC 6598 carrier-grade NAT
		"192.0.0.0/24",       // IETF protocol assignments
		"192.0.2.0/24",       // TEST-NET-1 documentation
		"198.18.0.0/15",      // RFC 2544 benchmarking
		"198.51.100.0/24",    // TEST-NET-2 documentation
		"203.0.113.0/24",     // TEST-NET-3 documentation
		"240.0.0.0/4",        // reserved (class E)
		"255.255.255.255/32", // limited broadcast
		"::/128",             // unspecified
		"64:ff9b::/96",       // NAT64 well-known prefix (maps to IPv4)
		"2001:db8::/32",      // documentation
		"100::/64",           // discard-only
	}
	nets := make([]*net.IPNet, 0, len(cidrs))
	for _, c := range cidrs {
		_, network, err := net.ParseCIDR(c)
		if err != nil {
			// The list is a compile-time constant; a parse failure is a
			// programming error worth surfacing loudly during tests.
			panic("respin: invalid blocked CIDR " + c + ": " + err.Error())
		}
		nets = append(nets, network)
	}
	return nets
}()

// isBlockedIP reports whether an already-resolved address is one the fetcher
// must refuse to connect to. It is the single chokepoint the SSRF dialer's
// Control hook and every URL guard funnel through, so DNS rebinding cannot slip
// an internal address past a hostname check: the address validated here is the
// exact one the socket connects to.
func isBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	// Collapse IPv4-mapped IPv6 (::ffff:a.b.c.d) to its IPv4 form so the IPv4
	// range checks below apply to addresses smuggled in IPv6 clothing.
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	if ip.IsLoopback() ||
		ip.IsUnspecified() ||
		ip.IsPrivate() || // RFC 1918 + fc00::/7 ULA
		ip.IsLinkLocalUnicast() || // 169.254.0.0/16 (incl. metadata) + fe80::/10
		ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() ||
		ip.IsMulticast() {
		return true
	}
	for _, network := range blockedCIDRs {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}
