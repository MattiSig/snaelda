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
  'landing.respin.link': 'Already have a website? Re-spin it',

  'respin.nav.home': 'Back to snaelda',
  'respin.eyebrow': 'Re-spin',
  'respin.title': 'Re-spin your current site',
  'respin.subtitle':
    'Paste the address of your existing website. Snaelda reads it, keeps your words and brand, and spins up a fresh draft — no signup.',
  'respin.form.placeholder': 'https://yourwebsite.is',
  'respin.form.ariaLabel': 'Your current website address',
  'respin.form.submit': 'Re-spin my site',
  'respin.form.submitBusy': 'Spinning...',
  'respin.form.helper':
    'A first draft from your current site, ready to edit. Not a perfect copy — a head start you can shape.',
  'respin.error.invalidUrl': 'Enter a full website address, like https://your-site.is',
  'respin.error.generic': 'Something went wrong. Please try again.',
  'respin.error.busy':
    "Re-spin is busy right now. Give it a minute and try again.",
  'respin.error.rateLimited':
    "That's a lot of re-spins from your network. Please try again shortly.",
  'respin.error.failed':
    "We couldn't re-spin that site. Try a different page, or start from a prompt instead.",
  'respin.progress.eyebrow': 'Spinning',
  'respin.progress.title': 'Re-spinning your site',
  'respin.progress.sourceLabel': 'From',
  'respin.progress.queued': 'Getting ready...',
  'respin.progress.fetching': 'Reading your site',
  'respin.progress.extracting': 'Pulling out your content',
  'respin.progress.composing': 'Spinning up the new draft',
  'respin.after.eyebrow': 'Your re-spun draft',
  'respin.after.badge': 'Draft preview',
  'respin.after.sourceLink': 'See your current site',
  'respin.after.loadingPreview': 'Loading your draft...',
  'respin.after.previewError': 'Could not load the draft preview.',
  'respin.after.note':
    "This is a first draft from your site's content. Keep it, then edit anything.",
  'respin.after.degradedNote':
    "We couldn't read everything, so a few sections are a starting point. Keep it and refine.",
  'respin.after.respinAnother': 'Re-spin another',
  'respin.after.keepIt': 'Sign up to keep it',
  'respin.degraded.eyebrow': 'A head start',
  'respin.degraded.title': "We couldn't read everything",
  'respin.degraded.body':
    "We couldn't pull enough from that site, so here's a head start you can build on.",
  'respin.degraded.cta': 'Continue with a head start',
  'respin.degraded.busy': 'Opening workspace...',
  'respin.degraded.retry': 'Try another site',
  'respin.claim.title': 'Keep this site',
  'respin.claim.body':
    "We'll save this draft to a free workspace and open it in the editor. You can add an email later to publish.",
  'respin.claim.confirm': 'Keep it & open editor',
  'respin.claim.busy': 'Saving...',
  'respin.claim.cancel': 'Not yet',
  'respin.claim.error': 'Could not keep this site. Please try again.',

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
  'landing.hero.titleLead': 'Þinn vefur,',
  'landing.hero.titleAccent': 'spunninn upp á mínútum',
  'landing.hero.subtitle':
    'Lýstu hvernig vefurinn á að vera, hvað þú gerir. Snælda spinnur upp fyrsta uppkastið. Fyrir þig að fínpússa og birta.',
  'landing.error.restore': 'Ekki tókst að endurheimta vinnusvæðið',
  'landing.error.start': 'Ekki tókst að opna vinnusvæðið',
  'landing.form.placeholder': 'Kósí kaffihús í Reykjavík...',
  'landing.form.ariaLabel': 'Lýstu því hvað þið gerið',
  'landing.form.submitOpening': 'Opna vinnusvæði...',
  'landing.form.submitAnother': 'Spinna aðra síðu',
  'landing.form.submitStart': 'Spinna',
  'landing.chips.label': 'Settu tóninn',
  'landing.chips.warmLocal': 'Hlýlegt og staðbundið',
  'landing.chips.booking': 'Bókun',
  'landing.chips.simple': 'Ekki flækja það',
  'landing.chips.premium': 'Gordjöss',
  'landing.form.helper':
    'Fyrsta uppkastið er ekki lokaorðið. Þegar það er komið heldurðu áfram að móta síðuna í skoðunar ham eða með AI-fínpússun.',
  'landing.consent.prefix': 'Með því að halda áfram samþykkir þú',
  'landing.consent.terms': 'skilmálana',
  'landing.consent.and': 'og',
  'landing.consent.privacy': 'persónuverndarstefnuna',
  'landing.demo.eyebrow': 'Rokkurinn vefur vef',
  'landing.demo.caption':
    'Alvöru vara, alvöru útkoma. Við styttum aðeins biðina svo þú starir ekki á hleðslutákn.',
  'landing.spins.eyebrow': 'Svona virkar Snælda',
  'landing.spins.lead':
    'Lýstu hugmyndinni. Snælda spinnur fyrsta uppkast: síður, uppbyggingu, texta, allt heila klabbið.',
  'landing.spins.mid': 'Lagaðu það sem þér finnst óklárt í einföldum ritli.',
  'landing.spins.tail':
    'Gefðu út vefinn þegar þú ert klár. Kláraðu dæmið með því að beina þínu léni að nýja vefnum!',
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
    'Minni fyrirtæki sem þurfa nýja vefsíðu núna, ekki nýtt eilífðar verkefni.',
  'landing.footer.terms': 'Skilmálar',
  'landing.footer.privacy': 'Persónuvernd',
  'landing.returning.eyebrow': 'Vinnusvæðið þitt bíður',
  'landing.returning.fallbackTrial': 'Þitt prufuvinnusvæðið',
  'landing.returning.fallbackWorkspace': 'Þitt vinnusvæðið',
  'landing.returning.cta': 'Halda áfram',
  'landing.respin.link': 'Ertu með vef nú þegar? Spinnaðu hann upp á nýtt',

  'respin.nav.home': 'Til baka á snaelda',
  'respin.eyebrow': 'Endurspuni',
  'respin.title': 'Spinnaðu núverandi vef upp á nýtt',
  'respin.subtitle':
    'Límdu inn slóðina á vefnum þínum. Snælda les hann, heldur textanum og útlitinu þínu og spinnur ferskt uppkast — engin innskráning.',
  'respin.form.placeholder': 'https://vefurinnþinn.is',
  'respin.form.ariaLabel': 'Slóðin á núverandi vef',
  'respin.form.submit': 'Spinna vefinn minn',
  'respin.form.submitBusy': 'Spinn...',
  'respin.form.helper':
    'Fyrsta uppkast úr núverandi vef, tilbúið til að fínpússa. Ekki nákvæm eftirmynd — forskot sem þú mótar áfram.',
  'respin.error.invalidUrl': 'Sláðu inn heila vefslóð, t.d. https://vefurinn-þinn.is',
  'respin.error.generic': 'Eitthvað fór úrskeiðis. Reyndu aftur.',
  'respin.error.busy':
    'Endurspuni er upptekinn í augnablikinu. Prófaðu aftur eftir smá stund.',
  'respin.error.rateLimited':
    'Þetta eru ansi margir endurspunar frá netinu þínu. Prófaðu aftur fljótlega.',
  'respin.error.failed':
    'Okkur tókst ekki að spinna þennan vef. Prófaðu aðra síðu eða byrjaðu frá lýsingu.',
  'respin.progress.eyebrow': 'Spinn',
  'respin.progress.title': 'Spinn vefinn þinn',
  'respin.progress.sourceLabel': 'Frá',
  'respin.progress.queued': 'Geri klárt...',
  'respin.progress.fetching': 'Les vefinn þinn',
  'respin.progress.extracting': 'Dreg út efnið þitt',
  'respin.progress.composing': 'Spinn nýja uppkastið',
  'respin.after.eyebrow': 'Þitt endurspunna uppkast',
  'respin.after.badge': 'Uppkast',
  'respin.after.sourceLink': 'Sjá núverandi vef',
  'respin.after.loadingPreview': 'Hleð uppkastinu þínu...',
  'respin.after.previewError': 'Ekki tókst að hlaða uppkastinu.',
  'respin.after.note':
    'Þetta er fyrsta uppkast úr efninu þínu. Haltu því og fínpússaðu svo hvað sem er.',
  'respin.after.degradedNote':
    'Við náðum ekki öllu, svo nokkrir hlutar eru bara byrjunarreitur. Haltu því og fínpússaðu.',
  'respin.after.respinAnother': 'Spinna annan',
  'respin.after.keepIt': 'Skráðu þig til að halda því',
  'respin.degraded.eyebrow': 'Forskot',
  'respin.degraded.title': 'Við náðum ekki öllu',
  'respin.degraded.body':
    'Við náðum ekki nógu miklu af vefnum, svo hér er forskot sem þú getur byggt á.',
  'respin.degraded.cta': 'Halda áfram með forskot',
  'respin.degraded.busy': 'Opna vinnusvæði...',
  'respin.degraded.retry': 'Prófa annan vef',
  'respin.claim.title': 'Haltu þessari síðu',
  'respin.claim.body':
    'Við vistum uppkastið á frítt vinnusvæði og opnum það í ritlinum. Þú getur bætt við netfangi síðar til að birta.',
  'respin.claim.confirm': 'Halda og opna ritil',
  'respin.claim.busy': 'Vista...',
  'respin.claim.cancel': 'Ekki strax',
  'respin.claim.error': 'Ekki tókst að halda síðunni. Reyndu aftur.',

  'login.error.magicLink': 'Ekki tókst að senda innskráningar link',
  'login.eyebrow': 'Aðgangur að ritli',
  'login.title': 'Skráðu þig inn með töfra link',
  'login.emailLabel': 'Netfang',
  'login.helper':
    'Ef þetta netfang á nú þegar vinnusvæði sendum við einnota innskráningartengil.',
  'login.submitSending': 'Sendi link...',
  'login.submit': 'Senda innskráningar link',
  'login.consent.prefix': 'Með því að halda áfram samþykkir þú',
  'login.consent.terms': 'notkunarskilmálana',
  'login.consent.and': 'og',
  'login.consent.privacy': 'persónuverndarstefnuna',

  'restore.error': 'Ekki tókst að endurheimta vinnusvæðið',
  'restore.inProgress': 'Endurheimti vinnusvæðið þitt...',
  'restore.missingKey': 'Endurheimtar link vantar lykil.',
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
