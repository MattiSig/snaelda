package respin

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

const sampleHTML = `<!doctype html>
<html lang="is">
<head>
  <title>Hárgreiðslustofan Klippt</title>
  <meta name="description" content="Klipping og litun í hjarta Reykjavíkur.">
  <meta name="theme-color" content="#7A3E48">
  <meta property="og:image" content="/images/hero.jpg">
  <link rel="icon" href="/favicon.ico">
  <link rel="apple-touch-icon" href="/touch.png">
  <script>console.log('tracking')</script>
  <style>.x{color:red}</style>
</head>
<body>
  <header><nav><a href="/thjonusta">Þjónusta</a><a href="/hafa-samband">Hafa samband</a></nav></header>
  <main>
    <h1>Velkomin á stofuna</h1>
    <p>Við bjóðum upp á klippingu, litun og blástur fyrir alla fjölskylduna í notalegu umhverfi.</p>
    <p>Bókaðu tíma í dag og láttu okkur dekra við þig frá toppi til táar með fagmennsku.</p>
    <div style="display:none">hidden secret coupon code</div>
  </main>
  <footer><p>Kreaddu okkur á Facebook</p></footer>
  <a href="https://external.example.com/off-site">off site</a>
  <a href="mailto:hello@klippt.is">email</a>
</body>
</html>`

func TestExtractPage(t *testing.T) {
	base, _ := url.Parse("https://klippt.is/")
	page := extractPage([]byte(sampleHTML), base)

	if page.Title != "Hárgreiðslustofan Klippt" {
		t.Errorf("title = %q", page.Title)
	}
	if !strings.Contains(page.Description, "Klipping og litun") {
		t.Errorf("description = %q", page.Description)
	}
	if page.Meta.ThemeColor != "#7A3E48" {
		t.Errorf("theme color = %q", page.Meta.ThemeColor)
	}
	if page.Meta.OGImage != "https://klippt.is/images/hero.jpg" {
		t.Errorf("og image = %q", page.Meta.OGImage)
	}
	if page.Meta.Lang != "is" {
		t.Errorf("lang = %q", page.Meta.Lang)
	}
	if len(page.Meta.IconHrefs) != 2 {
		t.Errorf("icon hrefs = %v", page.Meta.IconHrefs)
	}

	// Main content is kept; boilerplate and hidden text are dropped.
	if !strings.Contains(page.Text, "klippingu, litun og blástur") {
		t.Errorf("main copy missing: %q", page.Text)
	}
	for _, boiler := range []string{"tracking", "color:red", "hidden secret coupon", "Kreaddu okkur"} {
		if strings.Contains(page.Text, boiler) {
			t.Errorf("boilerplate leaked (%q) into %q", boiler, page.Text)
		}
	}

	// Same-origin links are resolved absolute; off-site and non-web dropped.
	assertContains(t, page.Links, "https://klippt.is/thjonusta")
	assertContains(t, page.Links, "https://klippt.is/hafa-samband")
	assertContains(t, page.Links, "https://external.example.com/off-site")
	for _, l := range page.Links {
		if strings.HasPrefix(l, "mailto:") {
			t.Errorf("mailto link should be dropped: %q", l)
		}
	}
}

func TestPageSufficiency(t *testing.T) {
	base, _ := url.Parse("https://klippt.is/")
	rich := extractPage([]byte(sampleHTML), base)
	if !rich.Sufficient() {
		t.Errorf("rich page (%d words) should be sufficient", rich.WordCount)
	}

	thin := extractPage([]byte("<html><body><main>Too short</main></body></html>"), base)
	if thin.Sufficient() {
		t.Errorf("thin page (%d words) should be insufficient", thin.WordCount)
	}
}

func TestFetchPageRejectsNonHTML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"not":"html"}`))
	}))
	defer srv.Close()

	_, err := testFetcher().FetchPage(context.Background(), srv.URL)
	var cte *ContentTypeError
	if !errors.As(err, &cte) {
		t.Fatalf("got %v, want ContentTypeError", err)
	}
}

func TestFetchPageStatusError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := testFetcher().FetchPage(context.Background(), srv.URL)
	var se *FetchStatusError
	if !errors.As(err, &se) {
		t.Fatalf("got %v, want FetchStatusError", err)
	}
	if se.StatusCode != 404 {
		t.Fatalf("status = %d", se.StatusCode)
	}
}

func assertContains(t *testing.T, haystack []string, needle string) {
	t.Helper()
	for _, h := range haystack {
		if h == needle {
			return
		}
	}
	t.Errorf("expected %q in %v", needle, haystack)
}
