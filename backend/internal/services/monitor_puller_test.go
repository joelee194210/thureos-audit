package services

import (
	"net"
	"testing"
)

// isPrivateOrReservedIP gates every outbound request the puller and the
// schema-detection probe make to a user-configured PullURL — this is the
// entire SSRF defense, so each address class it must block (and the public
// addresses it must let through) gets an explicit case.
func TestIsPrivateOrReservedIP(t *testing.T) {
	cases := []struct {
		name string
		ip   string
		want bool
	}{
		{"loopback v4", "127.0.0.1", true},
		{"loopback v6", "::1", true},
		{"loopback v4-mapped v6", "::ffff:127.0.0.1", true},
		{"rfc1918 10/8", "10.0.0.5", true},
		{"rfc1918 172.16/12", "172.16.5.1", true},
		{"rfc1918 192.168/16", "192.168.1.1", true},
		{"unique local v6", "fd00::1", true},
		{"link-local v4 incl. cloud metadata", "169.254.169.254", true},
		{"link-local v6", "fe80::1", true},
		{"unspecified v4", "0.0.0.0", true},
		{"unspecified v6", "::", true},
		{"0.0.0.0/8 non-zero host", "0.1.2.3", true},
		{"cgnat 100.64.0.0/10 low end", "100.64.0.1", true},
		{"cgnat 100.64.0.0/10 mid", "100.100.100.100", true},
		{"cgnat 100.64.0.0/10 high end", "100.127.255.254", true},
		{"just below cgnat range", "100.63.255.255", false},
		{"just above cgnat range", "100.128.0.1", false},
		{"public v4", "8.8.8.8", false},
		{"public v4 2", "1.1.1.1", false},
		{"public v6", "2606:4700:4700::1111", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ip := net.ParseIP(tc.ip)
			if ip == nil {
				t.Fatalf("no pude parsear %q como IP", tc.ip)
			}
			if got := isPrivateOrReservedIP(ip); got != tc.want {
				t.Errorf("isPrivateOrReservedIP(%s) = %v, want %v", tc.ip, got, tc.want)
			}
		})
	}
}
