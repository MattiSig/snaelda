// Hand-rolled locale dictionary for the marketing/landing surface.
//
// At ~50 strings across two locales a typed catalog is the right size for the
// Iceland phase (Spec 22); a full i18n framework only earns its keep once the
// builder UI is localized (hundreds of strings). The `en` catalog is the typed
// source of truth: `is` must satisfy the same `Strings` shape, so TypeScript
// fails the build if a key is added to one catalog and not the other.

export const LOCALES = ['is', 'en'] as const

export type Locale = (typeof LOCALES)[number]

// Iceland is the beachhead: the marketing surface defaults to Icelandic and
// only switches to English on an explicit signal (Spec 22).
export const DEFAULT_LOCALE: Locale = 'is'

// The Icelandic landing copy is written natively, not translated — the natural
// small-business register a Reykjavík shop or contractor would actually use,
// keeping the "serious result, un-serious process" voice through the switch.
const en = {
  'landing.nav.openWorkspace': 'Open workspace',
  'landing.nav.login': 'Log in',
  'landing.hero.titleLead': 'Spin up a website.',
  'landing.hero.titleAccent': 'A real one.',
  'landing.hero.subtitle':
    'Describe what you do. Snaelda lays down a real first draft, ready to refine, tweak, and publish.',
  'landing.error.restore': 'Could not restore that workspace',
  'landing.error.start': 'Could not start a workspace',
  'landing.form.placeholder': 'A cozy pottery studio in Portland...',
  'landing.form.ariaLabel': 'Describe your business',
  'landing.form.submitOpening': 'Opening workspace...',
  'landing.form.submitAnother': 'Start another site',
  'landing.form.submitStart': 'Spin my site',
  'landing.chips.label': 'Add direction',
  'landing.chips.warmLocal': 'Make it warm and local',
  'landing.chips.booking': 'Include booking',
  'landing.chips.simple': 'Keep it simple',
  'landing.chips.premium': 'Make it feel premium',
  'landing.form.helper':
    'The first draft is not the final word. Once it lands, keep shaping the site from the preview or the AI refine panel.',
  'landing.consent.prefix': 'By continuing, you agree to our',
  'landing.consent.terms': 'Terms',
  'landing.consent.and': 'and',
  'landing.consent.privacy': 'Privacy Policy',
  'landing.demo.eyebrow': 'See it spin',
  'landing.demo.caption':
    "Real product, real output. We only trimmed the loading wait so you're not watching a spinner.",
  'landing.spins.eyebrow': 'How it spins',
  'landing.spins.lead':
    'Prompt your idea. Snaelda lays down a real first draft: pages, structure, copy, the works.',
  'landing.spins.mid': 'Tweak whatever feels off in a lightweight editor.',
  'landing.spins.tail':
    'Publish when it feels right. Point your domain when it feels like home.',
  'landing.madeFor.prefix': 'Made for',
  'landing.madeFor.shops': 'shops',
  'landing.madeFor.studios': 'studios',
  'landing.madeFor.services': 'services',
  'landing.madeFor.contractors': 'contractors',
  'landing.madeFor.sideProjects': 'side projects',
  'landing.madeFor.everythingElse': 'everything in between',
  'landing.madeFor.sep': ', ',
  'landing.madeFor.lastSep': ', and ',
  'landing.madeFor.end': '. ',
  'landing.madeFor.tail':
    'Small operations that need a website, not a website project.',
  'landing.footer.terms': 'Terms',
  'landing.footer.privacy': 'Privacy',
  'landing.returning.eyebrow': 'Your workspace is waiting',
  'landing.returning.fallbackTrial': 'Your trial workspace',
  'landing.returning.fallbackWorkspace': 'Your workspace',
  'landing.returning.cta': 'Continue editing',

  'login.error.magicLink': 'Could not send magic link',
  'login.eyebrow': 'Builder access',
  'login.title': 'Log in by magic link',
  'login.emailLabel': 'Email',
  'login.helper':
    'If this email already owns a workspace, we’ll send a one-time sign-in link.',
  'login.submitSending': 'Sending link...',
  'login.submit': 'Send magic link',
  'login.consent.prefix': 'By continuing, you agree to our',
  'login.consent.terms': 'Terms of Use',
  'login.consent.and': 'and',
  'login.consent.privacy': 'Privacy Policy',

  'restore.error': 'Could not restore that workspace',
  'restore.inProgress': 'Restoring your workspace...',
  'restore.missingKey': 'Workspace recovery link is missing a key.',
  'restore.eyebrow': 'Workspace recovery',
} as const

export type StringKey = keyof typeof en

export type Strings = Record<StringKey, string>

const is: Strings = {
  'landing.nav.openWorkspace': 'Opna vinnusvæði',
  'landing.nav.login': 'Skrá inn',
  'landing.hero.titleLead': 'Spunnin vefsíða.',
  'landing.hero.titleAccent': 'Alvöru vefur.',
  'landing.hero.subtitle':
    'Lýstu því sem þú gerir. Snælda spinnur alvöru fyrsta uppkast, tilbúið til að fínpússa, breyta og birta.',
  'landing.error.restore': 'Ekki tókst að endurheimta vinnusvæðið',
  'landing.error.start': 'Ekki tókst að opna vinnusvæði',
  'landing.form.placeholder': 'Notaleg leirlistastofa í Reykjavík...',
  'landing.form.ariaLabel': 'Lýstu fyrirtækinu þínu',
  'landing.form.submitOpening': 'Opna vinnusvæði...',
  'landing.form.submitAnother': 'Spinna aðra síðu',
  'landing.form.submitStart': 'Spinna síðuna mína',
  'landing.chips.label': 'Bættu við stefnu',
  'landing.chips.warmLocal': 'Hafðu það hlýlegt og staðbundið',
  'landing.chips.booking': 'Bættu við bókun',
  'landing.chips.simple': 'Haltu því einföldu',
  'landing.chips.premium': 'Láttu það líta glæsilega út',
  'landing.form.helper':
    'Fyrsta uppkastið er ekki lokaorðið. Þegar það er komið heldurðu áfram að móta síðuna í forskoðuninni eða með AI-fínpússun.',
  'landing.consent.prefix': 'Með því að halda áfram samþykkir þú',
  'landing.consent.terms': 'skilmálana',
  'landing.consent.and': 'og',
  'landing.consent.privacy': 'persónuverndarstefnuna',
  'landing.demo.eyebrow': 'Sjáðu hana spinnast',
  'landing.demo.caption':
    'Alvöru vara, alvöru útkoma. Við styttum aðeins biðina svo þú starir ekki á hleðslutákn.',
  'landing.spins.eyebrow': 'Svona spinnst hún',
  'landing.spins.lead':
    'Lýstu hugmyndinni. Snælda spinnur alvöru fyrsta uppkast: síður, uppbyggingu, texta, allt heila klabbið.',
  'landing.spins.mid': 'Lagaðu það sem þér finnst óklárt í einföldum ritli.',
  'landing.spins.tail':
    'Birtu þegar það er tilbúið. Beindu léninu þínu þegar hún er orðin heimili.',
  'landing.madeFor.prefix': 'Gerð fyrir',
  'landing.madeFor.shops': 'búðir',
  'landing.madeFor.studios': 'stofur',
  'landing.madeFor.services': 'þjónustu',
  'landing.madeFor.contractors': 'verktaka',
  'landing.madeFor.sideProjects': 'hliðarverkefni',
  'landing.madeFor.everythingElse': 'allt þar á milli',
  'landing.madeFor.sep': ', ',
  'landing.madeFor.lastSep': ' og ',
  'landing.madeFor.end': '. ',
  'landing.madeFor.tail':
    'Lítil fyrirtæki sem þurfa vefsíðu, ekki heilt vefsíðuverkefni.',
  'landing.footer.terms': 'Skilmálar',
  'landing.footer.privacy': 'Persónuvernd',
  'landing.returning.eyebrow': 'Vinnusvæðið þitt bíður',
  'landing.returning.fallbackTrial': 'Prufuvinnusvæðið þitt',
  'landing.returning.fallbackWorkspace': 'Vinnusvæðið þitt',
  'landing.returning.cta': 'Halda áfram að breyta',

  'login.error.magicLink': 'Ekki tókst að senda innskráningartengil',
  'login.eyebrow': 'Aðgangur að ritli',
  'login.title': 'Skráðu þig inn með töfratengli',
  'login.emailLabel': 'Netfang',
  'login.helper':
    'Ef þetta netfang á nú þegar vinnusvæði sendum við einnota innskráningartengil.',
  'login.submitSending': 'Sendi tengil...',
  'login.submit': 'Senda töfratengil',
  'login.consent.prefix': 'Með því að halda áfram samþykkir þú',
  'login.consent.terms': 'notkunarskilmálana',
  'login.consent.and': 'og',
  'login.consent.privacy': 'persónuverndarstefnuna',

  'restore.error': 'Ekki tókst að endurheimta vinnusvæðið',
  'restore.inProgress': 'Endurheimti vinnusvæðið þitt...',
  'restore.missingKey': 'Endurheimtartengilinn vantar lykil.',
  'restore.eyebrow': 'Endurheimt vinnusvæðis',
}

const catalogs: Record<Locale, Strings> = { is, en }

/** Look up a localized string. Falls back to the `en` catalog for safety. */
export function t(locale: Locale, key: StringKey): string {
  const catalog = catalogs[locale] ?? catalogs.en
  return catalog[key] ?? en[key]
}

/** Bind a locale once and return a `t`-style lookup for that locale. */
export function translator(locale: Locale): (key: StringKey) => string {
  return (key) => t(locale, key)
}
