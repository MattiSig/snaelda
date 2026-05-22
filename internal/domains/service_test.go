package domains

import (
	"errors"
	"testing"
)

func TestValidateCustomHostname(t *testing.T) {
	tests := []struct {
		name      string
		hostname  string
		base      string
		want      string
		wantError error
	}{
		{
			name:     "normalizes valid hostname",
			hostname: "WWW.Example.COM.",
			base:     "sites.snaelda.test",
			want:     "www.example.com",
		},
		{
			name:      "rejects hosted base domain suffix",
			hostname:  "demo.sites.snaelda.test",
			base:      "sites.snaelda.test",
			wantError: ErrReservedHostname,
		},
		{
			name:      "rejects url schemes",
			hostname:  "https://example.com",
			base:      "sites.snaelda.test",
			wantError: ErrInvalidHostname,
		},
		{
			name:      "rejects single label hostnames",
			hostname:  "localhost",
			base:      "sites.snaelda.test",
			wantError: ErrInvalidHostname,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := validateCustomHostname(test.hostname, test.base)
			if test.wantError != nil {
				if err == nil || !errors.Is(err, test.wantError) {
					t.Fatalf("expected error %v, got %v", test.wantError, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("validate hostname: %v", err)
			}
			if got != test.want {
				t.Fatalf("expected %q, got %q", test.want, got)
			}
		})
	}
}

func TestVerificationHelpers(t *testing.T) {
	if got := verificationHostname("Example.com"); got != "_snaelda-verify.example.com" {
		t.Fatalf("expected verification hostname, got %q", got)
	}
	if got := verificationValue("token123"); got != "snaelda-site-verification=token123" {
		t.Fatalf("expected verification value, got %q", got)
	}
}
