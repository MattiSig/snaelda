package respin

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// respinSite builds a small multi-page site for discovery tests. Each content
// page carries enough words to pass the sufficiency threshold.
func respinSite() *http.ServeMux {
	body := func(title, extra string) string {
		return fmt.Sprintf(`<html lang="is"><head><title>%s</title></head><body><main>
			<h1>%s</h1>
			<p>Við bjóðum fjölbreytta þjónustu fyrir alla fjölskylduna í notalegu og fallegu umhverfi allan ársins hring.</p>
			<p>%s Hafðu samband og bókaðu tíma hjá okkur strax í dag með einföldum hætti og fljótt.</p>
			</main></body></html>`, title, title, extra)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<html lang="is"><head><title>Forsíða</title></head><body><main>
			<h1>Velkomin</h1>
			<p>Við bjóðum fjölbreytta þjónustu fyrir alla fjölskylduna í notalegu og fallegu umhverfi allan ársins hring.</p>
			<p>Komdu við hjá okkur og upplifðu fagmennsku, hlýju og persónulega þjónustu sem heldur þér ánægðum aftur og aftur um ókomna tíð.</p>
			<nav>
			  <a href="/um-okkur">Um okkur</a>
			  <a href="/thjonusta">Þjónusta</a>
			  <a href="/hafa-samband">Hafa samband</a>
			  <a href="/skra.pdf">Verðskrá PDF</a>
			  <a href="/leynt">Leynt</a>
			</nav>
			</main></body></html>`)
	})
	mux.HandleFunc("/um-okkur", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, body("Um okkur", "Stofan okkar hefur þjónað hverfinu í mörg ár."))
	})
	mux.HandleFunc("/thjonusta", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, body("Þjónusta", "Klipping, litun og blástur."))
	})
	mux.HandleFunc("/hafa-samband", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, body("Hafa samband", "Sími 555-1234."))
	})
	mux.HandleFunc("/leynt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, body("Leynt", "Þessi síða er bönnuð í robots."))
	})
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "User-agent: *\nDisallow: /leynt\n")
	})
	return mux
}

func TestFetchSiteDiscovers(t *testing.T) {
	srv := httptest.NewServer(respinSite())
	defer srv.Close()

	site, err := testFetcher().FetchSite(context.Background(), srv.URL, 5)
	if err != nil {
		t.Fatalf("FetchSite: %v", err)
	}
	if site.FetchMode != ModePlain {
		t.Fatalf("fetch mode = %q", site.FetchMode)
	}
	if site.Root.Title != "Forsíða" {
		t.Fatalf("root title = %q", site.Root.Title)
	}

	got := map[string]bool{}
	for _, p := range site.Pages {
		got[p.Title] = true
	}
	for _, want := range []string{"Um okkur", "Þjónusta", "Hafa samband"} {
		if !got[want] {
			t.Errorf("expected discovered page %q, got %v", want, got)
		}
	}
	// robots.txt disallows /leynt; the PDF is not a page.
	if got["Leynt"] {
		t.Error("robots-disallowed page was fetched")
	}
	for _, p := range site.Pages {
		if p.URL == srv.URL+"/skra.pdf" {
			t.Error("PDF link should not be fetched as a page")
		}
	}
}

func TestFetchSiteRespectsPageBudget(t *testing.T) {
	srv := httptest.NewServer(respinSite())
	defer srv.Close()

	site, err := testFetcher().FetchSite(context.Background(), srv.URL, 1)
	if err != nil {
		t.Fatalf("FetchSite: %v", err)
	}
	if len(site.Pages) != 1 {
		t.Fatalf("page budget of 1 not honoured: got %d pages", len(site.Pages))
	}
}

func TestFetchSiteInsufficientRoot(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<html><body><main>Of stutt.</main></body></html>")
	}))
	defer srv.Close()

	_, err := testFetcher().FetchSite(context.Background(), srv.URL, 5)
	if !errors.Is(err, ErrInsufficientContent) {
		t.Fatalf("got %v, want ErrInsufficientContent", err)
	}
}

func TestFetchSitePrioritizesHighValuePages(t *testing.T) {
	// With a budget of 2, the about/services pages should win over a generic
	// blog link by the scoring heuristic.
	mux := http.NewServeMux()
	page := func(title string) string {
		return fmt.Sprintf(`<html><head><title>%s</title></head><body><main>
			<p>Við bjóðum fjölbreytta þjónustu fyrir alla fjölskylduna í notalegu og fallegu umhverfi allan ársins hring án nokkurra vandræða.</p>
			<p>Komdu við hjá okkur og upplifðu fagmennsku, hlýju og persónulega þjónustu sem heldur þér ánægðum aftur og aftur um ókomna tíð.</p>
			</main></body></html>`, title)
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><head><title>Home</title></head><body><main>
			<p>Við bjóðum fjölbreytta þjónustu fyrir alla fjölskylduna í notalegu og fallegu umhverfi allan ársins hring án nokkurra vandræða.</p>
			<p>Komdu við hjá okkur og upplifðu fagmennsku, hlýju og persónulega þjónustu sem heldur þér ánægðum aftur og aftur um ókomna tíð.</p>
			<nav>
			  <a href="/blog/post-1">Blog</a>
			  <a href="/about">About</a>
			  <a href="/services">Services</a>
			</nav></main></body></html>`)
	})
	for _, p := range []string{"/blog/post-1", "/about", "/services"} {
		title := p
		mux.HandleFunc(p, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, page(title))
		})
	}
	srv := httptest.NewServer(mux)
	defer srv.Close()

	site, err := testFetcher().FetchSite(context.Background(), srv.URL, 2)
	if err != nil {
		t.Fatalf("FetchSite: %v", err)
	}
	if len(site.Pages) != 2 {
		t.Fatalf("expected 2 pages, got %d", len(site.Pages))
	}
	for _, p := range site.Pages {
		if p.Title == "/blog/post-1" {
			t.Errorf("blog page should not win over about/services within budget")
		}
	}
}
