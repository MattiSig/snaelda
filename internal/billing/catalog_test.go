package billing

import "testing"

func TestCatalogExposesISKPricesAndSitePlan(t *testing.T) {
	catalog := NewCatalog(
		map[string]string{"ISK": "price_site_isk"},
		map[string]string{"ISK": "price_pro_isk"},
		map[string]string{"ISK": "price_once_over_isk"},
	)

	response := catalog.Response()
	if len(response.Plans) != 2 {
		t.Fatalf("expected two plans, got %d", len(response.Plans))
	}
	site := response.Plans[0]
	if site.ID != "site" || site.Name != "Site" {
		t.Fatalf("expected first plan to be Site, got %q/%q", site.ID, site.Name)
	}
	if got := site.Prices["ISK"]; got != 2900 {
		t.Fatalf("expected site ISK price 2900, got %d", got)
	}
	if got := response.Plans[1].Prices["ISK"]; got != 6900 {
		t.Fatalf("expected pro ISK price 6900, got %d", got)
	}
	if got := response.OnceOverPrices["ISK"]; got != 13900 {
		t.Fatalf("expected once-over ISK price 13900, got %d", got)
	}
	// The response must not leak the internal Stripe price ids.
	if site.priceIDs != nil {
		t.Fatalf("expected response plan to drop price ids, got %#v", site.priceIDs)
	}
}

func TestCatalogResolvesPriceIDByCurrencyWithDefaultFallback(t *testing.T) {
	catalog := NewCatalog(
		map[string]string{"ISK": "price_site_isk"},
		map[string]string{"ISK": "price_pro_isk"},
		map[string]string{"ISK": "price_once_over_isk"},
	)

	if id, ok := catalog.PriceIDForPlan("site", "ISK"); !ok || id != "price_site_isk" {
		t.Fatalf("expected site ISK price id, got %q (ok=%v)", id, ok)
	}
	// An unconfigured currency falls back to the default (ISK) price id so a
	// checkout still resolves during the Iceland-only phase.
	if id, ok := catalog.PriceIDForPlan("site", "USD"); !ok || id != "price_site_isk" {
		t.Fatalf("expected default-currency fallback, got %q (ok=%v)", id, ok)
	}
	if id, ok := catalog.OnceOverPriceID(""); !ok || id != "price_once_over_isk" {
		t.Fatalf("expected once-over ISK price id, got %q (ok=%v)", id, ok)
	}
	// Every configured currency price id resolves back to its plan for webhook
	// plan derivation.
	if plan, ok := catalog.PlanByPriceID("price_pro_isk"); !ok || plan.ID != "pro" {
		t.Fatalf("expected price id to resolve to pro plan, got %q (ok=%v)", plan.ID, ok)
	}
}

func TestCatalogUnconfiguredPriceIDsAreUnavailable(t *testing.T) {
	catalog := NewCatalog(nil, nil, nil)
	if _, ok := catalog.PriceIDForPlan("site", "ISK"); ok {
		t.Fatal("expected no price id when none configured")
	}
	if _, ok := catalog.OnceOverPriceID("ISK"); ok {
		t.Fatal("expected no once-over price id when none configured")
	}
}

func TestFormatAmountHandlesZeroDecimalISK(t *testing.T) {
	amount, currency := formatAmount(2900, "isk")
	if currency != "ISK" {
		t.Fatalf("expected ISK currency, got %q", currency)
	}
	if amount != "2.900" {
		t.Fatalf("expected Icelandic-grouped 2.900, got %q", amount)
	}

	if got, _ := formatAmount(13900, "ISK"); got != "13.900" {
		t.Fatalf("expected 13.900, got %q", got)
	}
	if got, _ := formatAmount(950, "ISK"); got != "950" {
		t.Fatalf("expected 950 (no separator under 1000), got %q", got)
	}
}

func TestFormatAmountKeepsTwoDecimalsForUSD(t *testing.T) {
	amount, currency := formatAmount(2000, "usd")
	if currency != "USD" {
		t.Fatalf("expected USD currency, got %q", currency)
	}
	if amount != "20.00" {
		t.Fatalf("expected 20.00, got %q", amount)
	}
}
