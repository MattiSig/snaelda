import { createFileRoute } from '@tanstack/react-router'
import { LegalPage } from '@/components/LegalPage'

export const Route = createFileRoute('/terms')({
  head: () => ({
    meta: [
      { title: 'Terms of Use — Snaelda' },
      {
        name: 'description',
        content:
          'The terms that govern your use of Snaelda, the small-site workshop for solo operators and tiny businesses.',
      },
      { name: 'robots', content: 'index, follow' },
    ],
  }),
  component: TermsPage,
})

function TermsPage() {
  return (
    <LegalPage
      title="Terms of Use"
      lastUpdated="27 May 2026"
      intro={
        <p>
          These terms explain what you can expect from Snaelda and what we expect
          from you. We try to keep them short and human while still being clear
          about the legal bits.
        </p>
      }
    >
      <h2>1. About these terms</h2>
      <p>
        Snaelda is provided by <strong>[Company Legal Name] AB</strong>,
        a company registered in Sweden under organisation number{' '}
        <strong>[XXXXXX-XXXX]</strong>, with its registered address at{' '}
        <strong>[Registered Address, Postal Code, City, Sweden]</strong>{' '}
        (&quot;Snaelda&quot;, &quot;we&quot;, &quot;us&quot;).
      </p>
      <p>
        By creating an account, starting a workspace, or otherwise using
        snaelda.io and the related services (the &quot;Service&quot;), you
        agree to these Terms of Use (the &quot;Terms&quot;). If you do not
        agree, please do not use the Service.
      </p>

      <h2>2. The Service</h2>
      <p>
        Snaelda is a small-site workshop. It helps you draft, edit, preview,
        publish, and host a landing page or small website, including connecting
        a custom domain. The Service uses AI to generate first drafts of
        layout, structure, copy, and imagery based on prompts you provide.
      </p>
      <p>
        We may change, add, remove, or improve features at any time. We try not
        to break things that work, but we do not guarantee that any particular
        feature will be available forever.
      </p>

      <h2>3. Your account</h2>
      <ul>
        <li>You must be at least 16 years old to use the Service.</li>
        <li>
          You are responsible for keeping your login credentials and recovery
          keys safe. Activity that happens through your account is your
          responsibility.
        </li>
        <li>
          You must provide accurate account information and keep it up to date.
        </li>
        <li>
          If you use the Service on behalf of a company or other legal entity,
          you confirm that you are authorised to bind that entity to these Terms.
        </li>
      </ul>

      <h2>4. Subscriptions, payments, and refunds</h2>
      <p>
        Some parts of the Service are paid. Pricing and what is included is
        shown in the Service before you subscribe. Payments are processed by
        our payment provider, <strong>Stripe</strong>. By subscribing, you
        authorise us (via Stripe) to charge the applicable fees to your chosen
        payment method on a recurring basis until you cancel.
      </p>
      <p>
        You can cancel a subscription at any time from your billing settings.
        Cancellation takes effect at the end of the current billing period; we
        do not pro-rate refunds for unused time, except where required by law.
      </p>
      <p>
        <strong>Right of withdrawal (EU consumers).</strong> If you are a
        consumer in the EU, you normally have a 14-day right of withdrawal for
        services purchased online. Because the Service is delivered digitally
        and starts immediately, by subscribing you expressly request that we
        begin performance immediately and acknowledge that you lose your right
        of withdrawal once we have fully performed the service. For ongoing
        subscriptions, you may still cancel future renewals at any time.
      </p>
      <p>
        We may change pricing for future billing periods. We will give you
        reasonable notice before any price change affects you.
      </p>

      <h2>5. Your content</h2>
      <p>
        You keep all rights to the content you upload, type, or otherwise add
        to your sites (&quot;Your Content&quot;), including text, images, logos,
        and the resulting published site.
      </p>
      <p>
        You grant Snaelda a worldwide, non-exclusive, royalty-free licence to
        host, copy, process, transmit, display, and otherwise use Your Content
        only to the extent necessary to operate the Service for you — for
        example to render previews, publish your site, deliver it to visitors,
        back it up, and make it searchable to you within the workspace. This
        licence ends when you delete Your Content or close your account, except
        where we need to keep limited copies for legal, security, or backup
        reasons for a reasonable period.
      </p>
      <p>You are responsible for Your Content. You confirm that:</p>
      <ul>
        <li>
          you own it, or have all the rights and permissions needed to use it
          (including for any images, fonts, logos, or third-party material);
        </li>
        <li>
          it does not infringe anyone&apos;s rights, violate any law, or
          breach these Terms.
        </li>
      </ul>

      <h2>6. AI-generated content</h2>
      <p>
        Snaelda uses third-party AI models (currently provided by OpenAI) to
        generate site drafts, suggestions, and images. AI output can be
        inaccurate, biased, generic, or unexpectedly similar to other content.
      </p>
      <ul>
        <li>
          You are responsible for reviewing AI-generated content before
          publishing it.
        </li>
        <li>
          Do not rely on AI output for legal, medical, financial, safety, or
          other professional advice.
        </li>
        <li>
          You are responsible for making sure any AI-generated images, copy, or
          claims on your published site are accurate, lawful, and appropriate
          for your business.
        </li>
      </ul>
      <p>
        As between you and Snaelda, the output generated for your workspace is
        yours to use, subject to these Terms and the terms of the underlying AI
        provider.
      </p>

      <h2>7. Acceptable use</h2>
      <p>You agree not to use the Service to:</p>
      <ul>
        <li>
          publish or store content that is illegal, infringing, defamatory,
          hateful, harassing, sexually exploitative of minors, or that promotes
          violence or self-harm;
        </li>
        <li>
          impersonate any person or entity, or misrepresent your affiliation
          with one;
        </li>
        <li>
          send spam, run phishing or scam pages, distribute malware, or host
          adult content;
        </li>
        <li>
          attempt to break, probe, overload, or circumvent the security or
          access controls of the Service;
        </li>
        <li>
          scrape, mass-export, resell, or rebrand the Service or its outputs as
          your own competing product;
        </li>
        <li>
          use the Service in violation of Swedish law, EU law, or the laws of
          the country your site is targeted at.
        </li>
      </ul>

      <h2>8. Custom domains and third-party services</h2>
      <p>
        When you connect a custom domain or use a third-party integration, you
        are also subject to the terms of that domain registrar or third party.
        Snaelda is not responsible for outages, billing, or content of services
        we do not control.
      </p>

      <h2>9. Suspension and termination</h2>
      <p>
        You can stop using the Service and delete your account at any time. We
        may suspend or terminate your access if you materially breach these
        Terms, if we are required to do so by law, or if your use poses a
        security or legal risk to us or to other users. Where reasonable, we
        will give you notice and a chance to fix the issue first.
      </p>
      <p>
        On termination, your published sites may be taken offline and Your
        Content may be deleted from our systems after a short retention period.
        Please export anything you want to keep before you cancel.
      </p>

      <h2>10. Disclaimers</h2>
      <p>
        The Service is provided <strong>&quot;as is&quot;</strong> and{' '}
        <strong>&quot;as available&quot;</strong>. To the maximum extent
        permitted by law, we do not warrant that the Service will be
        uninterrupted, error-free, secure, or that AI output will be accurate
        or fit for any particular purpose. Nothing in these Terms limits any
        warranties or rights that cannot be excluded under mandatory consumer
        law applicable to you.
      </p>

      <h2>11. Limitation of liability</h2>
      <p>
        To the maximum extent permitted by law, Snaelda is not liable for any
        indirect, incidental, special, consequential, or punitive damages, or
        for any loss of profits, revenue, data, goodwill, or business
        opportunities, arising out of or related to your use of the Service.
      </p>
      <p>
        Our total liability to you for all claims relating to the Service in
        any 12-month period is limited to the greater of (a) the amounts you
        actually paid us for the Service in that period and (b) EUR 100.
      </p>
      <p>
        Nothing in these Terms limits liability for fraud, gross negligence,
        intentional misconduct, death or personal injury caused by negligence,
        or any other liability that cannot be limited under applicable law.
      </p>

      <h2>12. Indemnity</h2>
      <p>
        You agree to indemnify and hold Snaelda harmless from third-party
        claims, damages, and reasonable legal costs that arise from Your
        Content, your published sites, or your breach of these Terms or
        applicable law.
      </p>

      <h2>13. Changes to these Terms</h2>
      <p>
        We may update these Terms from time to time. If we make material
        changes, we will notify you through the Service or by email at least
        14 days before they take effect. Continuing to use the Service after
        the effective date means you accept the updated Terms.
      </p>

      <h2>14. Governing law and disputes</h2>
      <p>
        These Terms are governed by the laws of <strong>Sweden</strong>,
        without regard to conflict-of-law rules. Disputes will be resolved by
        the courts of Sweden, with <strong>Stockholms tingsrätt</strong> as
        the court of first instance.
      </p>
      <p>
        If you are a consumer resident in the EU, this does not deprive you of
        the protection afforded to you by mandatory provisions of the law of
        your country of residence, and you may also bring proceedings in the
        courts of that country. EU consumers may also use the European
        Commission&apos;s online dispute resolution platform at{' '}
        <a
          href="https://ec.europa.eu/consumers/odr"
          target="_blank"
          rel="noopener noreferrer"
        >
          ec.europa.eu/consumers/odr
        </a>
        .
      </p>

      <h2>15. Contact</h2>
      <p>
        Questions about these Terms? Email us at{' '}
        <a href="mailto:hello@snaelda.io">hello@snaelda.io</a>.
      </p>
    </LegalPage>
  )
}
