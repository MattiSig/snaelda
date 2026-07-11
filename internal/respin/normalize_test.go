package respin

import (
	"errors"
	"testing"
)

func TestNormalizeURL(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"bare host", "klippt.is", "https://klippt.is/"},
		{"adds root path", "https://klippt.is", "https://klippt.is/"},
		{"lowercases host", "https://Klippt.IS/Thjonusta", "https://klippt.is/Thjonusta"},
		{"strips fragment", "https://klippt.is/#tima", "https://klippt.is/"},
		{"trailing slash trimmed", "https://klippt.is/thjonusta/", "https://klippt.is/thjonusta"},
		{"strips default https port", "https://klippt.is:443/x", "https://klippt.is/x"},
		{"strips tracking params", "https://klippt.is/?utm_source=fb&utm_campaign=x&fbclid=abc", "https://klippt.is/"},
		{"keeps real params sorted", "https://klippt.is/?b=2&a=1&utm_medium=x", "https://klippt.is/?a=1&b=2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizeURL(tc.in)
			if err != nil {
				t.Fatalf("NormalizeURL(%q): %v", tc.in, err)
			}
			if got != tc.want {
				t.Fatalf("NormalizeURL(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestNormalizeURLStableCacheKey(t *testing.T) {
	// Two shares of the same page with different campaign tags and param order
	// must collapse to one cache key.
	a, err := NormalizeURL("https://klippt.is/tilbod?ref=fb&promo=sumar")
	if err != nil {
		t.Fatal(err)
	}
	b, err := NormalizeURL("https://klippt.is/tilbod/?promo=sumar&utm_source=insta")
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("cache keys differ: %q vs %q", a, b)
	}
}

func TestNormalizeURLRejects(t *testing.T) {
	cases := []struct {
		in      string
		wantErr error
	}{
		{"ftp://klippt.is", ErrDisallowedScheme},
		{"https://user:pass@klippt.is/", ErrCredentialsInURL},
		{"https://klippt.is:9000/", ErrDisallowedPort},
		{"   ", ErrEmptyURL},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			_, err := NormalizeURL(tc.in)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("NormalizeURL(%q) = %v, want %v", tc.in, err, tc.wantErr)
			}
		})
	}
}
