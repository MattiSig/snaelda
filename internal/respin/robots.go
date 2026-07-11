package respin

import (
	"context"
	"net/url"
	"strings"
)

// robotsRules holds the Disallow paths that apply to the re-spin fetcher for a
// single origin. Re-spin acts on the owner's explicit request, so it fetches the
// given URL regardless; robots.txt is consulted only for the small same-origin
// discovery beyond that URL (Spec 21 bot posture).
type robotsRules struct {
	disallow []string
}

// allowed reports whether path may be fetched under the collected rules. An
// empty disallow set (no robots.txt, unreadable, or no matching group) allows
// everything. Matching is longest-prefix over the applicable group, mirroring
// the de-facto robots.txt convention closely enough for one-shot discovery.
func (r *robotsRules) allowed(path string) bool {
	if r == nil || len(r.disallow) == 0 {
		return true
	}
	if path == "" {
		path = "/"
	}
	for _, rule := range r.disallow {
		if rule == "" {
			continue
		}
		if strings.HasPrefix(path, rule) {
			return false
		}
	}
	return true
}

// loadRobots fetches and parses /robots.txt for base's origin. Any failure
// (missing file, error, oversize) yields a permissive nil ruleset — robots.txt
// is advisory for our owner-initiated one-shot, not a hard gate.
func (f *Fetcher) loadRobots(ctx context.Context, base *url.URL) *robotsRules {
	robotsURL := &url.URL{Scheme: base.Scheme, Host: base.Host, Path: "/robots.txt"}
	res, err := f.Fetch(ctx, robotsURL.String(), FetchOptions{
		MaxBytes: 512 << 10, // robots files are small; cap defensively
		Accept:   "text/plain",
	})
	if err != nil || res.StatusCode >= 400 {
		return nil
	}
	return parseRobots(string(res.Body))
}

// parseRobots extracts the Disallow rules that apply to the re-spin user-agent.
// It collects rules from the most specific matching group: an explicit
// SnaeldaRespin group wins over the wildcard "*" group when present.
func parseRobots(body string) *robotsRules {
	const selfToken = "snaeldarespin"

	var (
		wildcard []string
		specific []string
		matchedSpecific bool
		// groupAgents tracks the user-agents declared for the current group so
		// consecutive User-agent lines share one rule block.
		groupAgents []string
		groupRules  []string
	)

	flush := func() {
		if len(groupAgents) == 0 {
			groupRules = nil
			return
		}
		for _, ua := range groupAgents {
			switch {
			case strings.Contains(ua, selfToken):
				specific = append(specific, groupRules...)
				matchedSpecific = true
			case ua == "*":
				wildcard = append(wildcard, groupRules...)
			}
		}
		groupAgents = nil
		groupRules = nil
	}

	sawRuleSinceAgent := false
	for _, raw := range strings.Split(body, "\n") {
		line := raw
		if idx := strings.IndexByte(line, '#'); idx >= 0 {
			line = line[:idx]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, value, ok := splitRobotsLine(line)
		if !ok {
			continue
		}
		switch key {
		case "user-agent":
			// A User-agent line after rules starts a new group.
			if sawRuleSinceAgent {
				flush()
				sawRuleSinceAgent = false
			}
			groupAgents = append(groupAgents, strings.ToLower(value))
		case "disallow":
			sawRuleSinceAgent = true
			if value != "" {
				groupRules = append(groupRules, value)
			}
		case "allow":
			sawRuleSinceAgent = true
			// Allow lines are honoured implicitly: paths not covered by a
			// Disallow prefix are already permitted, so we simply do not add a
			// Disallow for them.
		}
	}
	flush()

	rules := wildcard
	if matchedSpecific {
		rules = specific
	}
	if len(rules) == 0 {
		return nil
	}
	return &robotsRules{disallow: rules}
}

func splitRobotsLine(line string) (key, value string, ok bool) {
	idx := strings.IndexByte(line, ':')
	if idx < 0 {
		return "", "", false
	}
	key = strings.ToLower(strings.TrimSpace(line[:idx]))
	value = strings.TrimSpace(line[idx+1:])
	return key, value, true
}
