package netutil

import (
	"net/netip"
	"strings"
	"unicode/utf8"
)

const MaxUserAgentLength = 512

// NormalizeIP takes either a bare IP string or an address that may include a port
// (e.g. "192.0.2.4:1234" or "[2001:db8::1]:443") and returns the canonical IP
// portion without any zone identifiers. The second return value indicates if the
// address was successfully parsed as an IP address.
func NormalizeIP(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	if addrPort, err := netip.ParseAddrPort(raw); err == nil {
		addr := addrPort.Addr().WithZone("")
		if addr.IsValid() {
			return addr.String(), true
		}
	}
	if addr, err := netip.ParseAddr(raw); err == nil {
		addr = addr.WithZone("")
		if addr.IsValid() {
			return addr.String(), true
		}
	}
	// Handle bracketed IPv6 with a non-numeric port (e.g. "[::1]:port").
	if strings.HasPrefix(raw, "[") && strings.Contains(raw, "]") {
		host := raw[1:strings.LastIndex(raw, "]")]
		if addr, err := netip.ParseAddr(host); err == nil {
			addr = addr.WithZone("")
			if addr.IsValid() {
				return addr.String(), true
			}
		}
	}
	// Last resort: attempt to remove the trailing colon section and parse again.
	if idx := strings.LastIndex(raw, ":"); idx > 0 {
		host := raw[:idx]
		if addr, err := netip.ParseAddr(host); err == nil {
			addr = addr.WithZone("")
			if addr.IsValid() {
				return addr.String(), true
			}
		}
	}
	return raw, false
}

// TruncateUserAgent trims overly long user agents to MaxUserAgentLength runes.
func TruncateUserAgent(ua string) string {
	if ua == "" {
		return ""
	}
	if utf8.RuneCountInString(ua) <= MaxUserAgentLength {
		return ua
	}
	// Walk runes to avoid splitting multi-byte characters.
	var builder strings.Builder
	builder.Grow(len(ua))
	count := 0
	for _, r := range ua {
		builder.WriteRune(r)
		count++
		if count >= MaxUserAgentLength {
			break
		}
	}
	return builder.String()
}
