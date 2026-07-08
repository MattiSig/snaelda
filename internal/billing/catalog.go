package billing

import "strings"

// Iceland-first pricing (Spec 15). Amounts are in each currency's display unit;
// ISK is zero-decimal, so 2900 renders as "2.900 kr." not "29,00". Adding a
// currency is a data-only change: register its amount here and wire a price id
// through NewCatalog.
const (
	sitePlanPriceISK = 2900
	proPlanPriceISK  = 6900
	onceOverPriceISK = 13900
)

// defaultCurrency is the currency assumed for a workspace until per-workspace
// currency selection lands (the P1 workspaces.locale work). Phase 0 is
// Iceland-only, so every checkout resolves to ISK today.
const defaultCurrency = "ISK"

const gibibyte int64 = 1024 * 1024 * 1024

// subscriptionAmounts maps each subscription plan to its per-currency price. A
// currency is only offered for checkout when it has both an amount here and a
// configured Stripe price id.
var subscriptionAmounts = map[string]map[string]int{
	planSite: {defaultCurrency: sitePlanPriceISK},
	planPro:  {defaultCurrency: proPlanPriceISK},
}

// onceOverAmounts maps the once-over one-time purchase to its per-currency price.
var onceOverAmounts = map[string]int{defaultCurrency: onceOverPriceISK}

type Catalog struct {
	plansByID        map[string]PlanDefinition
	plansByPrice     map[string]PlanDefinition
	planOrder        []string
	onceOverPrices   map[string]int
	onceOverPriceIDs map[string]string
}

type PlanDefinition struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Prices maps an ISO-4217 currency code to the plan's monthly price in that
	// currency's display unit (zero-decimal for ISK).
	Prices                 map[string]int `json:"prices"`
	ActiveSiteLimit        int            `json:"activeSiteLimit"`
	MonthlyPromptLimit     int            `json:"monthlyPromptLimit"`
	AssetStorageLimitBytes int64          `json:"assetStorageLimitBytes"`
	CollectionLimit        int            `json:"collectionLimit"`
	CollectionEntryLimit   int            `json:"collectionEntryLimit"`
	CustomDomainsEnabled   bool           `json:"customDomainsEnabled"`
	PriorityOnceOver       bool           `json:"priorityOnceOver"`
	// priceIDs maps a currency code to the Stripe price id used at checkout.
	priceIDs map[string]string
}

type CatalogResponse struct {
	Plans          []PlanDefinition `json:"plans"`
	OnceOverPrices map[string]int   `json:"onceOverPrices"`
}

// NewCatalog builds the plan catalog from per-currency Stripe price id maps
// (currency code -> price id). Phase 0 passes ISK-only maps; extra currencies
// slot in as data once their price ids are provisioned.
func NewCatalog(sitePriceIDs, proPriceIDs, onceOverPriceIDs map[string]string) Catalog {
	plans := []PlanDefinition{
		{
			ID:                     planSite,
			Name:                   "Site",
			Prices:                 clonePrices(subscriptionAmounts[planSite]),
			ActiveSiteLimit:        3,
			MonthlyPromptLimit:     50,
			AssetStorageLimitBytes: 2 * gibibyte,
			CollectionLimit:        5,
			CollectionEntryLimit:   100,
			CustomDomainsEnabled:   true,
			PriorityOnceOver:       false,
			priceIDs:               normalizePriceIDs(sitePriceIDs),
		},
		{
			ID:                     planPro,
			Name:                   "Pro",
			Prices:                 clonePrices(subscriptionAmounts[planPro]),
			ActiveSiteLimit:        10,
			MonthlyPromptLimit:     200,
			AssetStorageLimitBytes: 20 * gibibyte,
			CollectionLimit:        25,
			CollectionEntryLimit:   1000,
			CustomDomainsEnabled:   true,
			PriorityOnceOver:       true,
			priceIDs:               normalizePriceIDs(proPriceIDs),
		},
	}

	catalog := Catalog{
		plansByID:        make(map[string]PlanDefinition, len(plans)),
		plansByPrice:     make(map[string]PlanDefinition, len(plans)),
		planOrder:        make([]string, 0, len(plans)),
		onceOverPrices:   clonePrices(onceOverAmounts),
		onceOverPriceIDs: normalizePriceIDs(onceOverPriceIDs),
	}
	for _, plan := range plans {
		catalog.plansByID[plan.ID] = plan
		catalog.planOrder = append(catalog.planOrder, plan.ID)
		for _, priceID := range plan.priceIDs {
			catalog.plansByPrice[priceID] = plan
		}
	}
	return catalog
}

func (c Catalog) PlanByID(planID string) (PlanDefinition, bool) {
	plan, ok := c.plansByID[strings.ToLower(strings.TrimSpace(planID))]
	return plan, ok
}

func (c Catalog) PlanByPriceID(priceID string) (PlanDefinition, bool) {
	plan, ok := c.plansByPrice[strings.TrimSpace(priceID)]
	return plan, ok
}

// PriceIDForPlan resolves the Stripe price id for a plan in the given currency,
// falling back to the default currency when the requested one is not offered.
func (c Catalog) PriceIDForPlan(planID string, currency string) (string, bool) {
	plan, ok := c.PlanByID(planID)
	if !ok {
		return "", false
	}
	return resolvePriceID(plan.priceIDs, currency)
}

// OnceOverPriceID resolves the Stripe price id for the once-over purchase in the
// given currency, falling back to the default currency.
func (c Catalog) OnceOverPriceID(currency string) (string, bool) {
	return resolvePriceID(c.onceOverPriceIDs, currency)
}

func (c Catalog) Response() CatalogResponse {
	plans := make([]PlanDefinition, 0, len(c.planOrder))
	for _, planID := range c.planOrder {
		plan := c.plansByID[planID]
		plan.priceIDs = nil
		plan.Prices = clonePrices(plan.Prices)
		plans = append(plans, plan)
	}
	return CatalogResponse{
		Plans:          plans,
		OnceOverPrices: clonePrices(c.onceOverPrices),
	}
}

func resolvePriceID(priceIDs map[string]string, currency string) (string, bool) {
	ccy := normalizeCurrency(currency)
	if id, ok := priceIDs[ccy]; ok && id != "" {
		return id, true
	}
	if id, ok := priceIDs[defaultCurrency]; ok && id != "" {
		return id, true
	}
	return "", false
}

func normalizePriceIDs(priceIDs map[string]string) map[string]string {
	normalized := make(map[string]string, len(priceIDs))
	for currency, id := range priceIDs {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			continue
		}
		normalized[normalizeCurrency(currency)] = trimmed
	}
	return normalized
}

func normalizeCurrency(currency string) string {
	trimmed := strings.ToUpper(strings.TrimSpace(currency))
	if trimmed == "" {
		return defaultCurrency
	}
	return trimmed
}

func clonePrices[V int | string](prices map[string]V) map[string]V {
	if prices == nil {
		return nil
	}
	cloned := make(map[string]V, len(prices))
	for k, v := range prices {
		cloned[k] = v
	}
	return cloned
}
