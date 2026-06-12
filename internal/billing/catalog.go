package billing

import "strings"

const (
	basicPlanPriceUSD = 19
	proPlanPriceUSD   = 49
	onceOverPriceUSD  = 99
)

const gibibyte int64 = 1024 * 1024 * 1024

type Catalog struct {
	plansByID    map[string]PlanDefinition
	plansByPrice map[string]PlanDefinition
	planOrder    []string
}

type PlanDefinition struct {
	ID                     string `json:"id"`
	Name                   string `json:"name"`
	MonthlyPriceUSD        int    `json:"monthlyPriceUsd"`
	ActiveSiteLimit        int    `json:"activeSiteLimit"`
	MonthlyPromptLimit     int    `json:"monthlyPromptLimit"`
	AssetStorageLimitBytes int64  `json:"assetStorageLimitBytes"`
	CollectionLimit        int    `json:"collectionLimit"`
	CollectionEntryLimit   int    `json:"collectionEntryLimit"`
	CustomDomainsEnabled   bool   `json:"customDomainsEnabled"`
	PriorityOnceOver       bool   `json:"priorityOnceOver"`
	priceID                string
}

type CatalogResponse struct {
	Plans            []PlanDefinition `json:"plans"`
	OnceOverPriceUSD int              `json:"onceOverPriceUsd"`
}

func NewCatalog(basicPriceID string, proPriceID string) Catalog {
	plans := []PlanDefinition{
		{
			ID:                     planBasic,
			Name:                   "Basic",
			MonthlyPriceUSD:        basicPlanPriceUSD,
			ActiveSiteLimit:        3,
			MonthlyPromptLimit:     50,
			AssetStorageLimitBytes: 2 * gibibyte,
			CollectionLimit:        5,
			CollectionEntryLimit:   100,
			CustomDomainsEnabled:   true,
			PriorityOnceOver:       false,
			priceID:                strings.TrimSpace(basicPriceID),
		},
		{
			ID:                     planPro,
			Name:                   "Pro",
			MonthlyPriceUSD:        proPlanPriceUSD,
			ActiveSiteLimit:        10,
			MonthlyPromptLimit:     200,
			AssetStorageLimitBytes: 20 * gibibyte,
			CollectionLimit:        25,
			CollectionEntryLimit:   1000,
			CustomDomainsEnabled:   true,
			PriorityOnceOver:       true,
			priceID:                strings.TrimSpace(proPriceID),
		},
	}

	catalog := Catalog{
		plansByID:    make(map[string]PlanDefinition, len(plans)),
		plansByPrice: make(map[string]PlanDefinition, len(plans)),
		planOrder:    make([]string, 0, len(plans)),
	}
	for _, plan := range plans {
		catalog.plansByID[plan.ID] = plan
		catalog.planOrder = append(catalog.planOrder, plan.ID)
		if plan.priceID != "" {
			catalog.plansByPrice[plan.priceID] = plan
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

func (c Catalog) PriceIDForPlan(planID string) (string, bool) {
	plan, ok := c.PlanByID(planID)
	if !ok || plan.priceID == "" {
		return "", false
	}
	return plan.priceID, true
}

func (c Catalog) Response() CatalogResponse {
	plans := make([]PlanDefinition, 0, len(c.planOrder))
	for _, planID := range c.planOrder {
		plan := c.plansByID[planID]
		plan.priceID = ""
		plans = append(plans, plan)
	}
	return CatalogResponse{
		Plans:            plans,
		OnceOverPriceUSD: onceOverPriceUSD,
	}
}
