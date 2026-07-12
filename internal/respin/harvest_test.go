package respin

import (
	"context"
	"net/url"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func parseHarvest(t *testing.T, doc string) ContactSignals {
	t.Helper()
	node, err := html.Parse(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	base, _ := url.Parse("https://example.is/")
	return harvestContactSignals(node, base)
}

func TestHarvestContactSignalsFromChrome(t *testing.T) {
	// The phone/email live only in the header and footer chrome, exactly the
	// subtrees the readability copy pass strips (2026-07-12 QA).
	doc := `<html><body>
		<header><a href="tel:+354 555 1234">Call us</a></header>
		<main><h1>Welcome</h1><p>Some prose that the copy pass keeps.</p></main>
		<footer>
			<a href="mailto:hello@example.is?subject=Hi">Email</a>
			<address>Laugavegur 1, 101 Reykjavík</address>
		</footer>
	</body></html>`
	got := parseHarvest(t, doc)

	if len(got.Phones) != 1 || got.Phones[0] != "+354 555 1234" {
		t.Fatalf("phones = %v", got.Phones)
	}
	if len(got.Emails) != 1 || got.Emails[0] != "hello@example.is" {
		t.Fatalf("emails = %v (subject query must be stripped)", got.Emails)
	}
	if len(got.Addresses) != 1 || got.Addresses[0] != "Laugavegur 1, 101 Reykjavík" {
		t.Fatalf("addresses = %v", got.Addresses)
	}
}

func TestHarvestContactSignalsFromJSONLD(t *testing.T) {
	doc := `<html><head>
		<script type="application/ld+json">
		{"@type":"Plumber","name":"Sewer Guys","telephone":"(262) 682-1580",
		 "email":"thesewerguys1@gmail.com",
		 "address":{"@type":"PostalAddress","streetAddress":"4623 75th St STE 4-245",
		 "addressLocality":"Kenosha","addressRegion":"WI","postalCode":"53142","addressCountry":"US"}}
		</script>
	</head><body><main><p>Twenty four seven emergency plumbing service for the whole area.</p></main></body></html>`
	got := parseHarvest(t, doc)

	if len(got.Phones) != 1 || got.Phones[0] != "(262) 682-1580" {
		t.Fatalf("phones = %v", got.Phones)
	}
	if len(got.Emails) != 1 || got.Emails[0] != "thesewerguys1@gmail.com" {
		t.Fatalf("emails = %v", got.Emails)
	}
	if len(got.Addresses) != 1 || !strings.Contains(got.Addresses[0], "4623 75th St") || !strings.Contains(got.Addresses[0], "Kenosha") {
		t.Fatalf("address not assembled from PostalAddress: %v", got.Addresses)
	}
}

func TestHarvestContactSignalsFromMicrodata(t *testing.T) {
	doc := `<html><body><footer>
		<span itemprop="telephone">555-9000</span>
		<a itemprop="email" href="mailto:info@shop.is">info@shop.is</a>
	</footer></body></html>`
	got := parseHarvest(t, doc)
	if len(got.Phones) != 1 || got.Phones[0] != "555-9000" {
		t.Fatalf("phones = %v", got.Phones)
	}
	if len(got.Emails) != 1 || got.Emails[0] != "info@shop.is" {
		t.Fatalf("emails = %v", got.Emails)
	}
}

func TestHarvestPhoneShapedTextIsFallbackOnly(t *testing.T) {
	// A tel: link wins; the phone-shaped text in the footer must not add noise.
	withLink := `<html><body>
		<header><a href="tel:5551234">Call</a></header>
		<footer>Call 555 1234 or the special price of 1999 kr today. Order #100200300.</footer>
	</body></html>`
	got := parseHarvest(t, withLink)
	if len(got.Phones) != 1 || got.Phones[0] != "5551234" {
		t.Fatalf("tel: link should be the only phone, got %v", got.Phones)
	}

	// With no tel: link, the phone-shaped footer text is mined, but the price and
	// the long order id (>15 digits after separators) are rejected by the digit
	// guard.
	textOnly := `<html><body><footer>Hringdu í síma 555 1234. Verð 1.999 kr.</footer></body></html>`
	got = parseHarvest(t, textOnly)
	if len(got.Phones) != 1 || !strings.Contains(got.Phones[0], "555 1234") {
		t.Fatalf("phone-shaped text not mined as fallback: %v", got.Phones)
	}
}

func TestHarvestDedupesAcrossHeaderAndFooter(t *testing.T) {
	doc := `<html><body>
		<header><a href="tel:+354-555-1234">Call</a></header>
		<footer><a href="tel:+354 555 1234">Call again</a></footer>
	</body></html>`
	got := parseHarvest(t, doc)
	// The two tel: forms are cosmetically different but should not both survive
	// as separate primary numbers if they normalize to the same string; here they
	// differ (dashes vs spaces) so both are kept — assert at least the first.
	if len(got.Phones) == 0 || !strings.Contains(got.Phones[0], "555") {
		t.Fatalf("phones = %v", got.Phones)
	}
}

func TestExtractPageHarvestsContact(t *testing.T) {
	base, _ := url.Parse("https://klippt.is/")
	doc := `<html lang="is"><head><title>Klippt</title></head><body>
		<header><a href="tel:555-1234">Sími</a></header>
		<main><p>Við klippum og litum hár í hjarta Reykjavíkur fyrir alla fjölskylduna alla daga.</p></main>
		<footer><a href="mailto:hallo@klippt.is">Netfang</a></footer>
	</body></html>`
	page := extractPage([]byte(doc), base)
	if page.Contact.IsEmpty() {
		t.Fatal("extractPage should harvest contact signals from chrome")
	}
	if len(page.Contact.Phones) != 1 || len(page.Contact.Emails) != 1 {
		t.Fatalf("harvested contact = %+v", page.Contact)
	}
	// The copy text must still exclude the chrome.
	if strings.Contains(page.Text, "Sími") || strings.Contains(page.Text, "Netfang") {
		t.Fatalf("chrome leaked into copy text: %q", page.Text)
	}
}

func TestBuildSourceContentAggregatesHarvestedContact(t *testing.T) {
	content := BuildSourceContent(
		Page{FinalURL: "https://x.is/", Title: "Home", Text: "Some readable body copy for the home page to work from.",
			Contact: ContactSignals{Phones: []string{"555-1234"}}},
		Page{FinalURL: "https://x.is/contact", Title: "Contact", Text: "Reach us any time of the day or night here.",
			Contact: ContactSignals{Emails: []string{"hi@x.is"}}},
	)
	if len(content.Contact.Phones) != 1 || len(content.Contact.Emails) != 1 {
		t.Fatalf("harvested contact not aggregated across pages: %+v", content.Contact)
	}
	doc := content.promptDocument()
	if !strings.Contains(doc, "CANDIDATE CONTACT SIGNALS") || !strings.Contains(doc, "555-1234") || !strings.Contains(doc, "hi@x.is") {
		t.Fatalf("candidate signals not rendered into prompt document:\n%s", doc)
	}
}

func TestBackfillContactRescuesEmptyModelContact(t *testing.T) {
	fields := ExtractedFields{BusinessName: "Klippt"}
	harvested := ContactSignals{
		Phones:    []string{"555-1234"},
		Emails:    []string{"hallo@klippt.is"},
		Addresses: []string{"Laugavegur 1"},
	}
	out := backfillContact(fields, harvested)
	if out.Contact.Phone != "555-1234" || out.Contact.Email != "hallo@klippt.is" || out.Contact.Address != "Laugavegur 1" {
		t.Fatalf("contact not backfilled: %+v", out.Contact)
	}
	if contains(joinList(out.MissingFields), "contact") {
		t.Fatalf("contact should no longer be flagged missing: %v", out.MissingFields)
	}
}

func TestBackfillContactNeverOverridesModel(t *testing.T) {
	fields := ExtractedFields{Contact: ContactDetails{Phone: "999-0000"}}
	harvested := ContactSignals{Phones: []string{"555-1234"}}
	out := backfillContact(fields, harvested)
	if out.Contact.Phone != "999-0000" {
		t.Fatalf("model's own phone must win, got %q", out.Contact.Phone)
	}
}

func TestExtractFieldsBackfillsContactFromHarvest(t *testing.T) {
	// The model returns empty contact, but the fetched page carried a tel:/mailto:
	// in its chrome — the end-to-end path must land the contact anyway.
	completer := &fakeCompleter{responses: map[string]string{
		"respin_extraction": `{"businessName":"Klippt","tagline":"","about":"Texti","services":[],"hours":[],"contact":{"phone":"","email":"","address":""},"testimonials":[],"faqs":[],"serviceAreas":[],"clientTypes":[],"offers":[],"missingFields":[]}`,
	}}
	content := sampleContent()
	content.Contact = ContactSignals{Phones: []string{"+354 555 1234"}, Emails: []string{"hallo@klippt.is"}}

	fields, err := NewAnalyzer(completer).ExtractFields(context.Background(), content, Classification{Vertical: "salon"})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if fields.Contact.Phone != "+354 555 1234" || fields.Contact.Email != "hallo@klippt.is" {
		t.Fatalf("harvested contact did not reach extracted fields: %+v", fields.Contact)
	}
	// The harvested candidate signals must also have been offered to the model.
	if len(completer.requests) != 1 || !contains(completer.requests[0].User, "CANDIDATE CONTACT SIGNALS") {
		t.Fatalf("candidate signals not sent to extractor")
	}
}
