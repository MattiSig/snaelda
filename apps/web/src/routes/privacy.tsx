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
      lastUpdated="27 May 2026"
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
        <strong>[Company Legal Name] AB</strong>, organisation number{' '}
        <strong>[XXXXXX-XXXX]</strong>, registered at{' '}
        <strong>[Registered Address, Postal Code, City, Sweden]</strong>{' '}
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

      <h2>3. Why we use your data, and the legal basis</h2>
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

      <h2>4. Sharing and subprocessors</h2>
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
          <strong>Hosting and storage providers</strong> used to run our
          servers, database, and object storage (including{' '}
          <strong>[Hosting Provider, e.g. Railway / AWS / Hetzner]</strong>).
        </li>
        <li>
          <strong>Email delivery providers</strong> for transactional emails
          such as login links, billing receipts, and account notices.
        </li>
        <li>
          <strong>Analytics and error-monitoring providers</strong> we may use
          to understand product usage and diagnose bugs.
        </li>
      </ul>
      <p>
        We may also disclose personal data when required by law, court order,
        or to protect the rights, property, or safety of Snaelda, our users,
        or others.
      </p>

      <h2>5. International transfers</h2>
      <p>
        Some of our subprocessors are located outside the European Economic
        Area (EEA), for example in the United States. When we transfer
        personal data outside the EEA, we rely on appropriate safeguards such
        as the European Commission&apos;s Standard Contractual Clauses, or on
        an adequacy decision where one applies.
      </p>

      <h2>6. How long we keep your data</h2>
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
          <strong>Backups:</strong> kept for a short rolling window, after
          which deletions propagate.
        </li>
      </ul>

      <h2>7. Your rights under the GDPR</h2>
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

      <h2>8. Cookies and similar technologies</h2>
      <p>
        We use a small number of strictly necessary cookies (and equivalent
        local storage) to keep you signed in, remember your colour-mode
        preference, and protect against abuse. These do not require consent
        under EU rules.
      </p>
      <p>
        If we later add analytics or marketing cookies that require consent,
        we will ask for your consent first through a cookie banner and give
        you a way to change your choice.
      </p>

      <h2>9. Security</h2>
      <p>
        We take reasonable technical and organisational measures to protect
        your data, including encryption in transit, restricted internal
        access, and secure password storage. No system is completely secure,
        so we cannot guarantee absolute security. If we become aware of a
        personal data breach affecting you, we will notify you and the
        relevant authority as required by law.
      </p>

      <h2>10. Children</h2>
      <p>
        Snaelda is not directed to children under 16. If you believe a child
        has provided us with personal data, please contact us and we will
        delete it.
      </p>

      <h2>11. Changes to this policy</h2>
      <p>
        We may update this policy as the Service evolves. If we make material
        changes, we will notify you through the Service or by email before
        they take effect. The &quot;Last updated&quot; date at the top tells
        you when the current version was published.
      </p>

      <h2>12. Contact</h2>
      <p>
        Privacy questions:{' '}
        <a href="mailto:privacy@snaelda.io">privacy@snaelda.io</a>
        <br />
        Postal address: <strong>[Registered Address, Postal Code, City, Sweden]</strong>
      </p>
    </LegalPage>
  )
}
