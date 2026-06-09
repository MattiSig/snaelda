// Display source for plan pricing. Stripe is the source of truth for billing;
// these constants must stay in sync with STRIPE_PRICE_BASIC, STRIPE_PRICE_PRO,
// and STRIPE_PRICE_ONCE_OVER until the API exposes pricing.

export type PlanFeature = {
  label: string
  included: boolean
}

export type PlanPricing = {
  id: 'basic' | 'pro'
  name: string
  tagline: string
  priceMonthly: number
  features: PlanFeature[]
  isRecommended?: boolean
  accent: string
}

export const PLANS: PlanPricing[] = [
  {
    id: 'basic',
    name: 'Basic',
    tagline: 'A small business site and its next rounds of edits.',
    priceMonthly: 19,
    features: [
      { label: '50 prompts / month', included: true },
      { label: '3 active sites', included: true },
      { label: '2 GB asset storage', included: true },
      { label: 'Custom domain', included: true },
      { label: 'Priority once-over slots', included: false },
    ],
    isRecommended: true,
    accent: 'var(--thread-gold)',
  },
  {
    id: 'pro',
    name: 'Pro',
    tagline: 'Multiple sites or heavier iteration on one.',
    priceMonthly: 49,
    features: [
      { label: '200 prompts / month', included: true },
      { label: '10 active sites', included: true },
      { label: '20 GB asset storage', included: true },
      { label: 'Custom domains', included: true },
      { label: 'Priority once-over slots', included: true },
    ],
    accent: 'var(--thread-teal)',
  },
]

export const ONCE_OVER_PRICE_USD = 99
