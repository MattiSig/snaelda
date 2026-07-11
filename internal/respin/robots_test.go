package respin

import "testing"

func TestParseRobotsWildcard(t *testing.T) {
	body := `
User-agent: *
Disallow: /admin
Disallow: /private/

User-agent: Googlebot
Disallow: /
`
	rules := parseRobots(body)
	if rules == nil {
		t.Fatal("expected rules")
	}
	if rules.allowed("/admin/settings") {
		t.Error("/admin should be disallowed")
	}
	if rules.allowed("/private/x") {
		t.Error("/private should be disallowed")
	}
	if !rules.allowed("/thjonusta") {
		t.Error("/thjonusta should be allowed")
	}
	// The Googlebot "/" block must not apply to us.
	if !rules.allowed("/") {
		t.Error("root should be allowed for our UA")
	}
}

func TestParseRobotsSpecificAgentWins(t *testing.T) {
	body := `
User-agent: *
Disallow: /

User-agent: SnaeldaRespin
Disallow: /secret
`
	rules := parseRobots(body)
	if rules == nil {
		t.Fatal("expected rules")
	}
	if rules.allowed("/secret/keys") {
		t.Error("/secret should be disallowed for our UA")
	}
	if !rules.allowed("/about") {
		t.Error("/about should be allowed since our specific group only blocks /secret")
	}
}

func TestParseRobotsEmpty(t *testing.T) {
	if parseRobots("") != nil {
		t.Error("empty robots should yield nil rules")
	}
	if parseRobots("# just a comment\nSitemap: https://x/sitemap.xml") != nil {
		t.Error("robots without disallow should yield nil rules")
	}
}

func TestRobotsAllowedNil(t *testing.T) {
	var r *robotsRules
	if !r.allowed("/anything") {
		t.Error("nil rules must allow everything")
	}
}
