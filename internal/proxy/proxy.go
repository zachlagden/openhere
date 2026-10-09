package proxy

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

type Trust struct {
	Prefixes []netip.Prefix
}

func (p Trust) trusted(addr netip.Addr) bool {
	addr = addr.Unmap().WithZone("")
	for _, prefix := range p.Prefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func peerAddr(r *http.Request) (netip.Addr, bool) {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}, false
	}
	return addr.Unmap(), true
}

func (p Trust) ClientIP(r *http.Request) netip.Addr {
	peer, ok := peerAddr(r)
	if !ok {
		return netip.Addr{}
	}
	if !p.trusted(peer) {
		return peer
	}
	hops := strings.Split(strings.Join(r.Header.Values("X-Forwarded-For"), ","), ",")
	for i := len(hops) - 1; i >= 0; i-- {
		hop, err := netip.ParseAddr(strings.TrimSpace(hops[i]))
		if err != nil {
			return peer
		}
		hop = hop.Unmap()
		if !p.trusted(hop) {
			return hop
		}
	}
	return peer
}

func (p Trust) IsHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	peer, ok := peerAddr(r)
	if !ok || !p.trusted(peer) {
		return false
	}
	values := strings.Split(strings.Join(r.Header.Values("X-Forwarded-Proto"), ","), ",")
	return strings.EqualFold(strings.TrimSpace(values[0]), "https")
}

func LimiterKey(addr netip.Addr) string {
	if !addr.IsValid() {
		return "unknown"
	}
	if addr.Is6() {
		prefix, err := addr.Prefix(64)
		if err == nil {
			return prefix.String()
		}
	}
	return addr.String()
}
