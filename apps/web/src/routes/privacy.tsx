import { createFileRoute } from '@tanstack/react-router'
import { LegalPage } from '@/components/LegalPage'

export const Route = createFileRoute('/privacy')({
  head: () => ({
    meta: [
      { title: 'Privacy Policy — Snaelda' },
      {
        name: 'description',
        content:
          'How Snaelda collects, uses, and protects your personal data under the GDPR.',
      },
      { name: 'robots', content: 'index, follow' },
    ],
  }),
  component: PrivacyPage,
})

function PrivacyPage() {
  return (
    <LegalPage
      title="Privacy Policy"
      lastUpdated="1 June 2026"
      intro={
        <p>
          This policy explains what personal data Snaelda collects, why we
          collect it, and what rights you have under the General Data
          Protection Regulation (GDPR) and Swedish data protection law.
        </p>
      }
    >
      <h2>1. Who is the controller</h2>
      <p>
        The controller of your personal data is{' '}
        <strong>Mosi hub AB</strong>, organisation number{' '}
        <strong>559577-7888</strong>, registered at{' '}
        <strong>Vallstigen 4, 431 69 Mölndal, Sweden</strong>{' '}
        (&quot;Snaelda&quot;, &quot;we&quot;, &quot;us&quot;).
      </p>
      <p>
        For any privacy question, contact us at{' '}
        <a href="mailto:privacy@snaelda.io">privacy@snaelda.io</a>.
      </p>

      <h2>2. Personal data we process</h2>
      <p>Depending on how you use the Service, we may process:</p>
      <ul>
        <li>
          <strong>Account data:</strong> email address, password hash, display
          name, recovery key, workspace identifiers.
        </li>
        <li>
          <strong>Content you create:</strong> prompts, site drafts, edits,
          uploaded media, published site content. This may include personal
          data about you or others if you choose to include it.
        </li>
        <li>
          <strong>Published-site and form data:</strong> content displayed on
          your published sites, form submissions sent through your sites, and
          basic page-view data for site analytics.
        </li>
        <li>
          <strong>Billing data:</strong> subscription plan, billing email,
          invoice history, country, and the last four digits of your card.
          Full card numbers are handled by Stripe and never touch our servers.
        </li>
        <li>
          <strong>Usage and technical data:</strong> IP address, device and
          browser type, language, pages visited, actions taken, timestamps,
          and error logs.
        </li>
        <li>
          <strong>Support communications:</strong> the content of emails or
          messages you send us.
        </li>
      </ul>
      <p>
        Please do not submit sensitive personal data, credentials, confidential
        secrets, medical information, financial account details, or other
        regulated information unless you have a lawful basis and all required
        rights to do so.
      </p>

      <h2>3. Customer sites and visitor data</h2>
      <p>
        For personal data in the content, forms, and visitor interactions on a
        site you create with Snaelda, you are normally the controller and
        Snaelda acts as your service provider or processor. This means you are
        responsible for having a lawful basis, privacy notice, and any required
        consents for the people whose data you collect through your site.
      </p>
      <p>
        Snaelda remains the controller for account administration, billing,
        security, platform logs, product emails, and other data we process for
        our own business and legal purposes.
      </p>

      <h2>4. Why we use your data, and the legal basis</h2>
      <ul>
        <li>
          <strong>To provide the Service</strong> (drafting, editing,
          publishing, hosting your site, handling logins and recovery) —
          performance of a contract (Art. 6(1)(b) GDPR).
        </li>
        <li>
          <strong>To process payments</strong> via Stripe and manage
          subscriptions — performance of a contract and legal obligation
          (Art. 6(1)(b) and (c) GDPR).
        </li>
        <li>
          <strong>To generate AI drafts</strong> by sending your prompts and
          related context to our AI provider (OpenAI) — performance of a
          contract (Art. 6(1)(b) GDPR).
        </li>
        <li>
          <strong>To handle published-site forms and basic site analytics</strong>{' '}
          for your workspace — performance of a contract (Art. 6(1)(b) GDPR)
          and, where we protect the platform against abuse, legitimate interest
          (Art. 6(1)(f) GDPR).
        </li>
        <li>
          <strong>To keep the Service secure and abuse-free</strong> (rate
          limiting, fraud detection, log retention) — our legitimate interest
          in protecting users and our systems (Art. 6(1)(f) GDPR).
        </li>
        <li>
          <strong>To comply with legal obligations</strong> such as
          bookkeeping, tax, and responding to lawful requests — Art. 6(1)(c)
          GDPR.
        </li>
        <li>
          <strong>To send product and service emails</strong> (transactional
          messages, important changes) — legitimate interest and contract.
          For marketing emails we rely on your consent, which you can
          withdraw at any time (Art. 6(1)(a) GDPR).
        </li>
      </ul>

      <h2>5. Sharing and subprocessors</h2>
      <p>
        We do not sell your personal data. We share it only with service
        providers that help us run Snaelda, under contracts that protect your
        data. Our main subprocessors are:
      </p>
      <ul>
        <li>
          <strong>Stripe Payments Europe, Ltd.</strong> — payment processing
          (Ireland / EU, with safeguards for any non-EU transfer).
        </li>
        <li>
          <strong>OpenAI Ireland Ltd. / OpenAI, L.L.C.</strong> — AI text and
          image generation. Prompts and related context are sent to OpenAI to
          produce drafts and suggestions.
        </li>
        <li>
          <strong>Resend</strong> — transactional email delivery, such as login
          links, billing notices, and account messages, when production email is
          enabled.
        </li>
        <li>
          <strong>Infrastructure and storage providers</strong> — hosting,
          database, and S3-compatible object storage used to run the Service,
          store assets, and deliver published sites.
        </li>
        <li>
          <strong>Imagery providers such as Pexels</strong> — starter image
          search and downloads where you use image suggestions.
        </li>
      </ul>
      <p>
        Snaelda&apos;s built-in site analytics are first-party and intentionally
        lightweight. If we add third-party analytics or error-monitoring tools
        that process personal data, we will update this policy.
      </p>
      <p>
        We may also disclose personal data when required by law, court order,
        or to protect the rights, property, or safety of Snaelda, our users,
        or others.
      </p>

      <h2>6. International transfers</h2>
      <p>
        Some of our subprocessors are located outside the European Economic
        Area (EEA), for example in the United States. When we transfer
        personal data outside the EEA, we rely on appropriate safeguards such
        as the European Commission&apos;s Standard Contractual Clauses, or on
        an adequacy decision where one applies.
      </p>

      <h2>7. How long we keep your data</h2>
      <ul>
        <li>
          <strong>Account and content:</strong> for as long as your account is
          active. When you delete your account, we delete or anonymise your
          personal data within 90 days, except where we need to keep it
          longer for legal reasons.
        </li>
        <li>
          <strong>Billing records:</strong> kept for seven (7) years to comply
          with Swedish bookkeeping law (Bokföringslagen).
        </li>
        <li>
          <strong>Security and access logs:</strong> typically up to 12
          months.
        </li>
        <li>
          <strong>Backups:</strong> kept in a short rolling window, normally no
          more than 30 days where under our control, after which deletions
          propagate.
        </li>
      </ul>

      <h2>8. Your rights under the GDPR</h2>
      <p>You have the right to:</p>
      <ul>
        <li>access the personal data we hold about you;</li>
        <li>have inaccurate data corrected;</li>
        <li>
          have your data erased where we no longer have a lawful basis to
          keep it;
        </li>
        <li>restrict or object to certain processing;</li>
        <li>
          receive a machine-readable copy of data you provided to us
          (portability);
        </li>
        <li>
          withdraw any consent you have given, without affecting the
          lawfulness of processing before withdrawal.
        </li>
      </ul>
      <p>
        To exercise any of these rights, email{' '}
        <a href="mailto:privacy@snaelda.io">privacy@snaelda.io</a>. We may
        need to verify your identity before we act on a request.
      </p>
      <p>
        You also have the right to lodge a complaint with the Swedish data
        protection authority,{' '}
        <strong>Integritetsskyddsmyndigheten (IMY)</strong>,{' '}
        <a
          href="https://www.imy.se"
          target="_blank"
          rel="noopener noreferrer"
        >
          imy.se
        </a>
        , or with the supervisory authority of your country of residence.
      </p>

      <h2>9. Cookies and similar technologies</h2>
      <p>
        We use a small number of strictly necessary cookies (and equivalent
        local storage) to keep you signed in, remember your colour-mode
        preference, and protect against abuse. These do not require consent
        under EU rules.
      </p>
      <p>
        You can block or delete cookies in your browser settings. If you block
        strictly necessary cookies, login, billing, editing, publishing, or
        security features may stop working.
      </p>
      <p>
        If we later add analytics or marketing cookies that require consent,
        we will ask for your consent first through a cookie banner and give
        you a way to change your choice.
      </p>

      <h2>10. Security</h2>
      <p>
        We take reasonable technical and organisational measures to protect
        your data, including encryption in transit, restricted internal
        access, and secure password storage. No system is completely secure,
        so we cannot guarantee absolute security. If we become aware of a
        personal data breach affecting you, we will notify you and the
        relevant authority as required by law.
      </p>

      <h2>11. Children</h2>
      <p>
        Snaelda is not directed to children under 16. If you believe a child
        has provided us with personal data, please contact us and we will
        delete it.
      </p>

      <h2>12. Changes to this policy</h2>
      <p>
        We may update this policy as the Service evolves. If we make material
        changes, we will notify you through the Service or by email before
        they take effect. The &quot;Last updated&quot; date at the top tells
        you when the current version was published.
      </p>

      <h2>13. Contact</h2>
      <p>
        Privacy questions:{' '}
        <a href="mailto:privacy@snaelda.io">privacy@snaelda.io</a>
        <br />
        Postal address: <strong>Vallstigen 4, 431 69 Mölndal, Sweden</strong>
      </p>
    </LegalPage>
  )
}
