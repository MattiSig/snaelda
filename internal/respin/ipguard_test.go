package respin

import (
	"errors"
	"net"
	"testing"
)

func TestIsBlockedIP(t *testing.T) {
	cases := []struct {
		name    string
		ip      string
		blocked bool
	}{
		{"loopback v4", "127.0.0.1", true},
		{"loopback v4 range", "127.9.9.9", true},
		{"private 10", "10.0.0.5", true},
		{"private 172.16", "172.16.5.5", true},
		{"private 192.168", "192.168.1.1", true},
		{"link-local", "169.254.1.1", true},
		{"cloud metadata", "169.254.169.254", true},
		{"cgn 100.64", "100.64.0.1", true},
		{"benchmark 198.18", "198.18.0.1", true},
		{"reserved class E", "240.0.0.1", true},
		{"broadcast", "255.255.255.255", true},
		{"this network", "0.0.0.0", true},
		{"unspecified v6", "::", true},
		{"loopback v6", "::1", true},
		{"ula v6", "fc00::1", true},
		{"link-local v6", "fe80::1", true},
		{"ipv4-mapped loopback", "::ffff:127.0.0.1", true},
		{"ipv4-mapped private", "::ffff:10.0.0.1", true},
		{"ipv4-mapped metadata", "::ffff:169.254.169.254", true},
		{"nat64 mapping metadata", "64:ff9b::a9fe:a9fe", true},
		{"documentation v6", "2001:db8::1", true},
		{"public v4", "93.184.216.34", false},
		{"public v4 google dns", "8.8.8.8", false},
		{"public v6", "2606:2800:220:1:248:1893:25c8:1946", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ip := net.ParseIP(tc.ip)
			if ip == nil {
				t.Fatalf("parse ip %q", tc.ip)
			}
			if got := isBlockedIP(ip); got != tc.blocked {
				t.Fatalf("isBlockedIP(%s) = %v, want %v", tc.ip, got, tc.blocked)
			}
		})
	}
}

func TestIsBlockedIPNil(t *testing.T) {
	if !isBlockedIP(nil) {
		t.Fatal("nil ip must be blocked")
	}
}

func TestGuardDialAddress(t *testing.T) {
	cases := []struct {
		name    string
		network string
		address string
		wantErr error
	}{
		{"public https", "tcp", "93.184.216.34:443", nil},
		{"public http", "tcp", "93.184.216.34:80", nil},
		{"loopback", "tcp", "127.0.0.1:443", ErrBlockedAddress},
		{"private", "tcp", "10.0.0.1:80", ErrBlockedAddress},
		{"metadata", "tcp", "169.254.169.254:80", ErrBlockedAddress},
		{"bad port", "tcp", "93.184.216.34:8080", ErrDisallowedPort},
		{"udp rejected", "udp", "93.184.216.34:443", ErrBlockedAddress},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := guardDialAddress(tc.network, tc.address, false)
			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected %v, got nil", tc.wantErr)
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("got %v, want %v", err, tc.wantErr)
			}
		})
	}
}
