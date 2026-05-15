package forms

import (
	"strings"
	"testing"
)

func TestAssessPayloadCleanSubmissionScoresZero(t *testing.T) {
	assessment := assessPayload(map[string]any{
		"name":    "Ada Lovelace",
		"email":   "ada@example.com",
		"message": "Hi, I would like to discuss a calmer studio site for our florist.",
	}, nil)
	if assessment.IsSpam() {
		t.Fatalf("expected clean payload to be accepted, got %#v", assessment)
	}
	if assessment.Score != 0 {
		t.Fatalf("expected score 0 for clean payload, got %v", assessment.Score)
	}
	if len(assessment.Signals) != 0 {
		t.Fatalf("expected no signals for clean payload, got %#v", assessment.Signals)
	}
}

func TestAssessPayloadHoneypotIsAutoSpam(t *testing.T) {
	assessment := assessPayload(map[string]any{
		"name":    "Ada",
		"email":   "ada@example.com",
		"message": "Hello",
	}, map[string]string{"hp_url": "http://spammer.example"})
	if !assessment.IsSpam() {
		t.Fatalf("expected honeypot match to be auto-spam, got %#v", assessment)
	}
	if !containsSignal(assessment.Signals, "honeypot:hp_url") {
		t.Fatalf("expected honeypot signal, got %#v", assessment.Signals)
	}
}

func TestAssessPayloadFlagsExcessiveLinks(t *testing.T) {
	message := "Check these https://a.example https://b.example https://c.example https://d.example https://e.example"
	assessment := assessPayload(map[string]any{
		"message": message,
	}, nil)
	if !containsSignal(assessment.Signals, "links:high") {
		t.Fatalf("expected links:high signal, got %#v", assessment.Signals)
	}
	if assessment.Score < 0.7 {
		t.Fatalf("expected elevated score for many links, got %v", assessment.Score)
	}
}

func TestAssessPayloadCombinedSignalsCrossSpamThreshold(t *testing.T) {
	message := "viagra casino offers at https://a.example https://b.example https://c.example https://d.example https://e.example"
	assessment := assessPayload(map[string]any{
		"message": message,
	}, nil)
	if !assessment.IsSpam() {
		t.Fatalf("expected combined keywords + many links to be auto-spam, got %#v", assessment)
	}
}

func TestAssessPayloadFlagsScriptMarkup(t *testing.T) {
	assessment := assessPayload(map[string]any{
		"message": "<script>alert('xss')</script>",
	}, nil)
	if !assessment.IsSpam() {
		t.Fatalf("expected script markup to be auto-spam, got %#v", assessment)
	}
}

func TestAssessPayloadKeywordsAccumulateButCap(t *testing.T) {
	assessment := assessPayload(map[string]any{
		"message": "We provide cheap viagra and cialis with casino bonuses and seo services. Buy followers now.",
	}, nil)
	if assessment.Score <= 0 {
		t.Fatalf("expected keyword hits to bump score, got %v", assessment.Score)
	}
	if assessment.Score > 1.0 {
		t.Fatalf("expected score to be clamped to 1.0, got %v", assessment.Score)
	}
}

func TestAssessPayloadFlagsRepeatedChars(t *testing.T) {
	assessment := assessPayload(map[string]any{
		"message": "loooooove this offer aaaaaaa",
	}, nil)
	if !containsSignal(assessment.Signals, "noise:repeated_chars") {
		t.Fatalf("expected repeated_chars signal, got %#v", assessment.Signals)
	}
}

func TestAssessPayloadFlagsAllCapsLongMessage(t *testing.T) {
	assessment := assessPayload(map[string]any{
		"message": "URGENT NOTICE READ THIS NOW MAKE SURE TO CALL BACK",
	}, nil)
	if !containsSignal(assessment.Signals, "casing:all_caps") {
		t.Fatalf("expected casing:all_caps signal, got %#v", assessment.Signals)
	}
}

func TestExtractHoneypotFieldsSeparatesKeys(t *testing.T) {
	cleaned, honeypot := extractHoneypotFields(map[string]any{
		"email":  "ada@example.com",
		"hp_url": "http://spammer.example",
		"HP_X":   "anything",
	})
	if _, ok := cleaned["email"]; !ok {
		t.Fatalf("expected legitimate fields to remain, got %#v", cleaned)
	}
	if _, ok := cleaned["hp_url"]; ok {
		t.Fatalf("expected hp_url to be stripped, got %#v", cleaned)
	}
	if _, ok := cleaned["HP_X"]; ok {
		t.Fatalf("expected case-insensitive honeypot stripping, got %#v", cleaned)
	}
	if honeypot["hp_url"] != "http://spammer.example" || honeypot["HP_X"] != "anything" {
		t.Fatalf("expected honeypot values to be captured, got %#v", honeypot)
	}
}

func TestHasRepeatedRunBoundary(t *testing.T) {
	if !hasRepeatedRun("aaaaaa", 6) {
		t.Fatal("expected exactly six matching chars to count")
	}
	if hasRepeatedRun("aaaaa", 6) {
		t.Fatal("expected five matching chars to be below threshold")
	}
	if hasRepeatedRun(strings.Repeat("ab", 10), 4) {
		t.Fatal("expected alternating chars to not trigger")
	}
}

func containsSignal(signals []string, target string) bool {
	for _, signal := range signals {
		if signal == target {
			return true
		}
	}
	return false
}
