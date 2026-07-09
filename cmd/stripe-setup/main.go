// stripe-setup creates the products, prices, and webhook endpoint Snaelda needs
// in Stripe, then prints the env vars to copy into .env. The script is
// idempotent: re-running it reuses existing resources matched by lookup key.
//
// Usage:
//
//	go run ./cmd/stripe-setup \
//	  -key sk_test_… \
//	  -webhook-url https://snaelda.app/api/billing/webhook
//
// Defaults to test mode. To run against a live key, pass -allow-live; you will
// be prompted to type "yes" before any writes happen.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/stripe/stripe-go/v85"
	"github.com/stripe/stripe-go/v85/price"
	"github.com/stripe/stripe-go/v85/product"
	"github.com/stripe/stripe-go/v85/webhookendpoint"
)

var webhookEvents = []string{
	"checkout.session.completed",
	"customer.subscription.created",
	"customer.subscription.updated",
	"customer.subscription.deleted",
	"invoice.paid",
	"invoice.payment_failed",
}

type planSpec struct {
	slug      string
	product   string
	lookupKey string
	amount    int64
	currency  string
	interval  string // empty for one-time
}

func main() {
	var (
		key            = flag.String("key", os.Getenv("STRIPE_SECRET_KEY"), "Stripe secret key (defaults to STRIPE_SECRET_KEY env var)")
		webhookURL     = flag.String("webhook-url", "http://localhost:8080/api/billing/webhook", "Public webhook URL the API can be reached at")
		siteAmount     = flag.Int64("site-amount", 2900, "Site plan monthly price in whole currency units (krónur, kronor, dollars)")
		proAmount      = flag.Int64("pro-amount", 6900, "Pro plan monthly price in whole currency units")
		onceOverAmount = flag.Int64("once-over-amount", 13900, "Once-over add-on price in whole currency units")
		currency       = flag.String("currency", "isk", "Three-letter currency code")
		allowLive      = flag.Bool("allow-live", false, "Required to run against a sk_live_… key")
		skipWebhook    = flag.Bool("skip-webhook", false, "Skip webhook endpoint creation (use when developing locally with `stripe listen`)")
	)
	flag.Parse()

	if strings.TrimSpace(*key) == "" {
		log.Fatal("missing Stripe key: pass -key or set STRIPE_SECRET_KEY")
	}

	live := strings.HasPrefix(*key, "sk_live_")
	mode := "TEST"
	if live {
		mode = "LIVE"
		if !*allowLive {
			log.Fatal("refusing to run against a live key without -allow-live")
		}
	}

	stripe.Key = *key

	ccy := strings.ToUpper(strings.TrimSpace(*currency))
	fmt.Printf("Snaelda Stripe setup — %s mode\n", mode)
	fmt.Printf("Webhook URL: %s\n", *webhookURL)
	fmt.Printf("Site:        %d %s/month\n", *siteAmount, ccy)
	fmt.Printf("Pro:         %d %s/month\n", *proAmount, ccy)
	fmt.Printf("Once-over:   %d %s one-time\n\n", *onceOverAmount, ccy)

	if live {
		fmt.Print("Live keys affect real customers. Type 'yes' to continue: ")
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		if strings.TrimSpace(response) != "yes" {
			log.Fatal("aborted")
		}
	}

	// Lookup keys are currency-scoped: reuse-by-lookup-key must never hand back
	// a price in a different currency (the unscoped keys once matched the
	// legacy USD prices when provisioning ISK).
	ccyKey := strings.ToLower(ccy)
	plans := []planSpec{
		{slug: "site", product: "Snaelda Site", lookupKey: "snaelda_site_monthly_" + ccyKey, amount: *siteAmount, currency: *currency, interval: string(stripe.PriceRecurringIntervalMonth)},
		{slug: "pro", product: "Snaelda Pro", lookupKey: "snaelda_pro_monthly_" + ccyKey, amount: *proAmount, currency: *currency, interval: string(stripe.PriceRecurringIntervalMonth)},
		{slug: "once_over", product: "Snaelda Once-over review", lookupKey: "snaelda_once_over_" + ccyKey, amount: *onceOverAmount, currency: *currency, interval: ""},
	}

	results := make(map[string]string, len(plans))
	for _, plan := range plans {
		id, err := ensurePrice(plan)
		if err != nil {
			log.Fatalf("ensure %s price: %v", plan.slug, err)
		}
		results[plan.slug] = id
	}

	fmt.Println()
	fmt.Println("Env vars to add to your .env (or Railway/Render/Fly secrets):")
	fmt.Printf("STRIPE_PRICE_SITE_%s=%s\n", ccy, results["site"])
	fmt.Printf("STRIPE_PRICE_PRO_%s=%s\n", ccy, results["pro"])
	fmt.Printf("STRIPE_PRICE_ONCE_OVER_%s=%s\n", ccy, results["once_over"])

	if !*skipWebhook {
		secret, reused, err := ensureWebhook(*webhookURL)
		if err != nil {
			log.Fatalf("ensure webhook: %v", err)
		}
		if reused {
			fmt.Println("# STRIPE_WEBHOOK_SECRET: reused existing endpoint — Stripe only reveals the signing")
			fmt.Println("#   secret at creation time. Roll it from the dashboard if you don't have it,")
			fmt.Println("#   or re-run with -skip-webhook and recreate by hand.")
		} else {
			fmt.Printf("STRIPE_WEBHOOK_SECRET=%s\n", secret)
		}
	} else {
		fmt.Println("# STRIPE_WEBHOOK_SECRET: skipped — for local dev use `stripe listen --forward-to localhost:8080/api/billing/webhook`")
		fmt.Println("#   and paste the whsec_… it prints into STRIPE_WEBHOOK_SECRET.")
	}
}

// stripeZeroDecimalCurrencies is Stripe's zero-decimal list: these currencies
// take unit_amount in whole units. ISK is deliberately NOT here — Stripe
// treats it as a backward-compatibility special case represented as a
// two-decimal amount whose decimals are always 00, so like ordinary
// two-decimal currencies it is multiplied by 100 (2.900 kr -> 290000).
var stripeZeroDecimalCurrencies = map[string]bool{
	"BIF": true, "CLP": true, "DJF": true, "GNF": true,
	"JPY": true, "KMF": true, "KRW": true, "MGA": true, "PYG": true, "RWF": true,
	"VND": true, "VUV": true, "XAF": true, "XOF": true, "XPF": true,
}

// stripeUnitAmount converts a price in whole currency units to the unit_amount
// Stripe expects for that currency.
func stripeUnitAmount(wholeUnits int64, currency string) int64 {
	if stripeZeroDecimalCurrencies[strings.ToUpper(strings.TrimSpace(currency))] {
		return wholeUnits
	}
	return wholeUnits * 100
}

func ensurePrice(plan planSpec) (string, error) {
	listIter := price.List(&stripe.PriceListParams{
		LookupKeys: stripe.StringSlice([]string{plan.lookupKey}),
	})
	for listIter.Next() {
		existing := listIter.Price()
		if !strings.EqualFold(string(existing.Currency), plan.currency) {
			return "", fmt.Errorf("lookup_key %s matched price %s in %s, want %s — refusing to reuse across currencies", plan.lookupKey, existing.ID, existing.Currency, plan.currency)
		}
		fmt.Printf("✓ %-10s  reuse  %s (lookup_key=%s)\n", plan.slug, existing.ID, plan.lookupKey)
		return existing.ID, nil
	}
	if err := listIter.Err(); err != nil {
		return "", err
	}

	prod, err := ensureProduct(plan.slug, plan.product)
	if err != nil {
		return "", err
	}

	params := &stripe.PriceParams{
		Product:    stripe.String(prod),
		UnitAmount: stripe.Int64(stripeUnitAmount(plan.amount, plan.currency)),
		Currency:   stripe.String(plan.currency),
		LookupKey:  stripe.String(plan.lookupKey),
	}
	if plan.interval != "" {
		params.Recurring = &stripe.PriceRecurringParams{
			Interval: stripe.String(plan.interval),
		}
	}

	created, err := price.New(params)
	if err != nil {
		return "", err
	}
	fmt.Printf("✓ %-10s  create %s\n", plan.slug, created.ID)
	return created.ID, nil
}

func ensureProduct(slug, name string) (string, error) {
	query := fmt.Sprintf("metadata['snaelda_product']:'%s'", slug)
	searchIter := product.Search(&stripe.ProductSearchParams{
		SearchParams: stripe.SearchParams{Query: query},
	})
	for searchIter.Next() {
		existing := searchIter.Product()
		return existing.ID, nil
	}
	if err := searchIter.Err(); err != nil {
		return "", err
	}

	created, err := product.New(&stripe.ProductParams{
		Name:     stripe.String(name),
		Metadata: map[string]string{"snaelda_product": slug},
	})
	if err != nil {
		return "", err
	}
	return created.ID, nil
}

func ensureWebhook(url string) (secret string, reused bool, err error) {
	iter := webhookendpoint.List(&stripe.WebhookEndpointListParams{})
	for iter.Next() {
		ep := iter.WebhookEndpoint()
		if ep.URL == url {
			fmt.Printf("✓ webhook    reuse  %s -> %s\n", ep.ID, ep.URL)
			return "", true, nil
		}
	}
	if err := iter.Err(); err != nil {
		return "", false, err
	}

	ep, err := webhookendpoint.New(&stripe.WebhookEndpointParams{
		URL:           stripe.String(url),
		EnabledEvents: stripe.StringSlice(webhookEvents),
	})
	if err != nil {
		return "", false, err
	}
	fmt.Printf("✓ webhook    create %s -> %s\n", ep.ID, ep.URL)
	return ep.Secret, false, nil
}
