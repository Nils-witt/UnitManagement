package server

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// realIP replaces r.RemoteAddr with the client's IP address (without a port)
// when the request came through one of the trusted proxies. The client is
// the rightmost X-Forwarded-For entry that is not itself a trusted proxy, or
// X-Real-IP when X-Forwarded-For is absent. Requests from any other peer are
// left alone, so clients can't spoof their address by sending the headers.
func realIP(trusted []netip.Prefix, next http.Handler) http.Handler {
	if len(trusted) == 0 {
		return next
	}
	isTrusted := func(addr netip.Addr) bool {
		for _, p := range trusted {
			if p.Contains(addr) {
				return true
			}
		}
		return false
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		peer, ok := parseIP(r.RemoteAddr)
		if ok && isTrusted(peer) {
			if client, ok := forwardedClient(r.Header, isTrusted); ok {
				r.RemoteAddr = client.String()
			}
		}
		next.ServeHTTP(w, r)
	})
}

// forwardedClient walks X-Forwarded-For from the nearest hop outwards and
// returns the first address not belonging to a trusted proxy. If every hop
// is trusted it returns the farthest one. It stops at a malformed entry,
// since nothing beyond it can be relied on.
func forwardedClient(h http.Header, isTrusted func(netip.Addr) bool) (netip.Addr, bool) {
	var hops []string
	for _, v := range h.Values("X-Forwarded-For") {
		hops = append(hops, strings.Split(v, ",")...)
	}
	if len(hops) == 0 {
		return parseIP(h.Get("X-Real-IP"))
	}

	var client netip.Addr
	for i := len(hops) - 1; i >= 0; i-- {
		addr, ok := parseIP(hops[i])
		if !ok {
			break
		}
		client = addr
		if !isTrusted(addr) {
			break
		}
	}
	return client, client.IsValid()
}

// parseIP accepts an IP address with or without a port ("1.2.3.4",
// "1.2.3.4:5678", "::1", "[::1]:5678").
func parseIP(s string) (netip.Addr, bool) {
	s = strings.TrimSpace(s)
	if host, _, err := net.SplitHostPort(s); err == nil {
		s = host
	}
	addr, err := netip.ParseAddr(strings.Trim(s, "[]"))
	if err != nil {
		return netip.Addr{}, false
	}
	return addr.Unmap().WithZone(""), true
}
