package respin

import (
	"net/url"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func parseHero(t *testing.T, doc string) SourceHero {
	t.Helper()
	node, err := html.Parse(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	base, _ := url.Parse("https://example.is/")
	return extractSourceHero(node, base)
}

func TestExtractSourceHeroTextOnly(t *testing.T) {
	hero := parseHero(t, `<html><body>
		<header><a href="/">Home</a></header>
		<section class="hero">
			<h1>Clogged Drain? We Fix Them 24/7</h1>
			<p>Fast, verified emergency plumbing across the whole city.</p>
			<a class="btn" href="/contact">Call now</a>
		</section>
		<main><p>Body copy about the business.</p></main>
	</body></html>`)

	if hero.Headline != "Clogged Drain? We Fix Them 24/7" {
		t.Fatalf("headline = %q", hero.Headline)
	}
	if hero.Subheadline != "Fast, verified emergency plumbing across the whole city." {
		t.Fatalf("subheadline = %q", hero.Subheadline)
	}
	if hero.CTALabel != "Call now" {
		t.Fatalf("cta = %q", hero.CTALabel)
	}
	if !hero.TextOnly {
		t.Fatalf("expected text-only hero, got image %q", hero.ImageURL)
	}
}

func TestExtractSourceHeroWithImage(t *testing.T) {
	hero := parseHero(t, `<html><body>
		<section id="masthead">
			<div class="section-background"><img src="/hero-photo.jpg" alt="Our team"></div>
			<h1>Handcrafted in Reykjavík</h1>
			<h2>Since 1994</h2>
			<a href="/shop">Shop the collection</a>
		</section>
	</body></html>`)

	if hero.Headline != "Handcrafted in Reykjavík" {
		t.Fatalf("headline = %q", hero.Headline)
	}
	if hero.Subheadline != "Since 1994" {
		t.Fatalf("subheadline = %q", hero.Subheadline)
	}
	if hero.CTALabel != "Shop the collection" {
		t.Fatalf("cta = %q", hero.CTALabel)
	}
	if hero.ImageURL != "https://example.is/hero-photo.jpg" {
		t.Fatalf("image = %q", hero.ImageURL)
	}
	if hero.TextOnly {
		t.Fatal("expected image-led hero, got text-only")
	}
}

func TestExtractSourceHeroCSSBackground(t *testing.T) {
	hero := parseHero(t, `<html><body>
		<section style="background-image: url('https://cdn.example.is/bg.png'); color:#fff">
			<h1>Welcome</h1>
		</section>
	</body></html>`)

	if hero.ImageURL != "https://cdn.example.is/bg.png" {
		t.Fatalf("image = %q", hero.ImageURL)
	}
	if hero.TextOnly {
		t.Fatal("a CSS background hero is not text-only")
	}
}

func TestExtractSourceHeroFallsBackToH1Ancestor(t *testing.T) {
	// No hero-named container: the first <section> holding the <h1> is the hero.
	hero := parseHero(t, `<html><body>
		<div class="wrapper">
			<section>
				<h1>Nordic Lens Studio</h1>
				<p>Portraits and weddings.</p>
			</section>
		</div>
	</body></html>`)

	if hero.Headline != "Nordic Lens Studio" {
		t.Fatalf("headline = %q", hero.Headline)
	}
	if hero.Subheadline != "Portraits and weddings." {
		t.Fatalf("subheadline = %q", hero.Subheadline)
	}
}

func TestExtractSourceHeroEmptyWithoutHeading(t *testing.T) {
	hero := parseHero(t, `<html><body><main><p>Just paragraphs, no heading.</p></main></body></html>`)
	if !hero.IsEmpty() {
		t.Fatalf("expected empty hero, got %+v", hero)
	}
}

func TestCSSBackgroundURL(t *testing.T) {
	cases := map[string]string{
		`background-image:url(/a.jpg)`:                     "/a.jpg",
		`background: url("https://x.is/b.png") no-repeat`:  "https://x.is/b.png",
		`background-image: url('c.webp')`:                  "c.webp",
		`color:#fff`:                                       "",
		`background-image:url(data:image/png;base64,AAAA)`: "",
	}
	for style, want := range cases {
		if got := cssBackgroundURL(style); got != want {
			t.Errorf("cssBackgroundURL(%q) = %q, want %q", style, got, want)
		}
	}
}

// TestSourceHeroEndToEndQAScenario reproduces the 2026-07-13 QA failure mode: a
// Squarespace-style source whose real hero photo lives in a section-background
// while og:image is the social/logo card. After extraction and scoring, the
// source's own hero photo must outrank the og:image, and the punchy source
// headline must survive as the extracted hero headline.
func TestSourceHeroEndToEndQAScenario(t *testing.T) {
	base, _ := url.Parse("https://mysewerguys.example/")
	page := extractPage([]byte(`<html lang="en"><head>
		<meta property="og:image" content="https://mysewerguys.example/social-card.png">
	</head><body>
		<header><img src="/logo.png" class="site-logo" alt="The Sewer Guys"></header>
		<section class="hero-section" data-section-id="1">
			<div class="section-background"><img src="/img/hero-truck.jpg" alt="Our service truck"></div>
			<h1>Clogged Drain? We Fix Them 24/7</h1>
			<p>Fast, verified emergency plumbing across the metro.</p>
			<a href="/contact" class="btn-primary">Call now</a>
		</section>
		<main><p>The Sewer Guys have served the metro for over a decade with honest, upfront pricing and round-the-clock emergency response.</p></main>
	</body></html>`), base)

	if page.Hero.Headline != "Clogged Drain? We Fix Them 24/7" {
		t.Fatalf("hero headline = %q", page.Hero.Headline)
	}
	if page.Hero.ImageURL != "https://mysewerguys.example/img/hero-truck.jpg" {
		t.Fatalf("hero image = %q", page.Hero.ImageURL)
	}

	candidates := heroCandidates(aggregate([]Page{page}))
	if len(candidates) == 0 {
		t.Fatal("expected hero candidates")
	}
	if candidates[0].url != "https://mysewerguys.example/img/hero-truck.jpg" {
		t.Fatalf("top hero candidate = %q, want the source hero photo, not the og:image", candidates[0].url)
	}
	// The og:image social card must rank strictly below the source hero photo.
	for i, c := range candidates {
		if c.url == "https://mysewerguys.example/social-card.png" && i == 0 {
			t.Fatal("og:image social card wrongly ranked first")
		}
	}
}

func TestExtractPageMarksHeroImageRegion(t *testing.T) {
	base, _ := url.Parse("https://example.is/")
	page := extractPage([]byte(`<html><body>
		<header><img src="/logo.png" class="logo"></header>
		<section class="hero"><div class="section-background"><img src="/hero.jpg"></div><h1>Big type here today</h1></section>
	</body></html>`), base)

	if page.Hero.Headline != "Big type here today" {
		t.Fatalf("hero headline = %q", page.Hero.Headline)
	}
	found := false
	for _, img := range page.Meta.Images {
		if img.URL == "https://example.is/hero.jpg" {
			found = true
			if img.Region != "hero" {
				t.Fatalf("hero image region = %q, want hero", img.Region)
			}
		}
	}
	if !found {
		t.Fatal("hero image not present in page images")
	}
}
