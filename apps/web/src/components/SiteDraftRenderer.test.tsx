import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { SiteDraft } from '@/lib/api';
import { SiteDraftRenderer } from './SiteDraftRenderer';

beforeEach(() => {
  vi.restoreAllMocks();
});

describe('SiteDraftRenderer', () => {
  it('renders visible blocks and resolves draft-page links to anchors', () => {
    render(<SiteDraftRenderer site={buildDraft()} />);

    const navLinks = within(
      screen.getByRole('navigation', { name: 'Site navigation' }),
    ).getAllByRole('link');
    expect(navLinks[0]?.getAttribute('href')).toBe('#page-home');
    expect(navLinks[1]?.getAttribute('href')).toBe('#page-contact');
    expect(
      screen.getByRole('link', { name: 'Book a visit' }).getAttribute('href'),
    ).toBe('#page-contact');
    expect(
      screen
        .getByAltText('Studio shelves in warm afternoon light')
        .getAttribute('src'),
    ).toBe('http://localhost:8080/api/assets/asset-hero/content');
    expect(screen.getByText('Friendly yarn for colder days.')).toBeTruthy();
    expect(
      screen.queryByText('This hidden block should never render.'),
    ).toBeNull();
  });

  it('resolves published links and narrows rendering to the selected page', () => {
    render(
      <SiteDraftRenderer
        site={buildDraft()}
        linkMode="published"
        selectedPageId="page-contact"
        showPageMeta={false}
        siteSlug="loom-light"
      />,
    );

    const navLinks = within(
      screen.getByRole('navigation', { name: 'Site navigation' }),
    ).getAllByRole('link');
    expect(navLinks[0]?.getAttribute('href')).toBe('/public/loom-light');
    expect(navLinks[1]?.getAttribute('href')).toBe(
      '/public/loom-light/contact',
    );
    expect(screen.queryByText('Home')).toBeNull();
    expect(screen.getByText('Contact')).toBeTruthy();
  });

  it('keeps published links root-relative when rendered on a hosted domain', () => {
    render(
      <SiteDraftRenderer
        site={buildDraft()}
        linkMode="published"
        siteSlug="loom-light"
        publishedBasePath=""
      />,
    );

    const navLinks = within(
      screen.getByRole('navigation', { name: 'Site navigation' }),
    ).getAllByRole('link');
    expect(navLinks[0]?.getAttribute('href')).toBe('/');
    expect(navLinks[1]?.getAttribute('href')).toBe('/contact');
    expect(
      screen.getByRole('link', { name: 'Book a visit' }).getAttribute('href'),
    ).toBe('/contact');
    expect(
      screen
        .getByAltText('Studio shelves in warm afternoon light')
        .getAttribute('src'),
    ).toBe(
      'http://localhost:8080/api/public/sites/loom-light/assets/asset-hero',
    );
  });

  it('renders the added MVP content blocks without fallback UI', () => {
    render(<SiteDraftRenderer site={buildExtendedDraft()} />);

    expect(screen.queryByText('Unsupported block')).toBeNull();
    expect(screen.getByText('Selected work')).toBeTruthy();
    expect(screen.getByText('What clients tend to notice')).toBeTruthy();
    expect(screen.getByText('Packages and starting points')).toBeTruthy();
    expect(
      screen.getByText('Questions people ask before they reach out'),
    ).toBeTruthy();
    expect(screen.getByText('People behind the work')).toBeTruthy();
    expect(
      screen
        .getAllByRole('link', { name: 'Contact' })
        .some((link) => link.getAttribute('href') === '#page-contact'),
    ).toBe(true);
  });

  it('renders the structured footer contact and a Made with Snælda link', () => {
    const draft: SiteDraft = {
      ...buildDraft(),
      site: { ...buildDraft().site, defaultLocale: 'is' },
      pages: [
        {
          id: 'page-home',
          title: 'Home',
          slug: '/',
          blocks: [
            {
              id: 'block-footer',
              type: 'footer',
              version: '1.0.0',
              props: {
                showBrand: true,
                showMadeWith: true,
                copyright: 'Copyright 2026 Fléttan',
                contact: {
                  address: {
                    street: 'Laugavegur 12',
                    city: 'Reykjavík',
                    postalCode: '101',
                    country: 'Ísland',
                  },
                  phone: '+354 555 1234',
                  email: 'hallo@flettan.is',
                  hours: [
                    { day: 'monday', opens: '09:00', closes: '17:00' },
                    { day: 'sunday', closed: true },
                  ],
                },
              },
              settings: {},
            },
          ],
        },
      ],
    };

    render(<SiteDraftRenderer site={draft} />);

    expect(screen.getByText('Laugavegur 12', { exact: false })).toBeTruthy();
    expect(screen.getByText('101 Reykjavík', { exact: false })).toBeTruthy();
    expect(
      screen.getByRole('link', { name: '+354 555 1234' }).getAttribute('href'),
    ).toBe('tel:+3545551234');
    // Icelandic weekday labels and closed marker.
    expect(screen.getByText('Mánudagur')).toBeTruthy();
    expect(screen.getByText('09:00–17:00')).toBeTruthy();
    expect(screen.getByText('Sunnudagur')).toBeTruthy();
    expect(screen.getByText('Lokað')).toBeTruthy();

    const madeWith = screen.getByRole('link', { name: 'Gert með Snældu' });
    expect(madeWith.getAttribute('href')).toBe('https://snaelda.io');
  });

  it('hides the Made with Snælda link when the owner opts out', () => {
    const draft: SiteDraft = {
      ...buildDraft(),
      pages: [
        {
          id: 'page-home',
          title: 'Home',
          slug: '/',
          blocks: [
            {
              id: 'block-footer',
              type: 'footer',
              version: '1.0.0',
              props: {
                showMadeWith: false,
                copyright: 'Copyright 2026 Loom & Light',
              },
              settings: {},
            },
          ],
        },
      ],
    };

    render(<SiteDraftRenderer site={draft} />);

    expect(screen.queryByText('Made with Snælda')).toBeNull();
    expect(screen.queryByText('Gert með Snældu')).toBeNull();
  });

  it('submits contact form blocks through the public forms API', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      status: 202,
      json: async () => ({
        status: 'accepted',
        message: 'Thanks. Your message is on its way.',
      }),
    });
    vi.stubGlobal('fetch', fetchMock);

    render(<SiteDraftRenderer site={buildContactFormDraft()} />);

    fireEvent.change(screen.getByLabelText('Email *'), {
      target: { value: 'ada@example.com' },
    });
    fireEvent.change(screen.getByLabelText('Message *'), {
      target: { value: 'Need a warmer site direction.' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Send inquiry' }));

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledTimes(1);
    });
    expect(fetchMock.mock.calls[0]?.[0].toString()).toContain(
      '/api/public/forms/site-1/block-contact/submit',
    );
    expect(
      screen.getByText('Thanks. Your message is on its way.'),
    ).toBeTruthy();
  });

  it('supports block wrappers for editor chrome without rendering hidden blocks', () => {
    render(
      <SiteDraftRenderer
        site={buildDraft()}
        renderBlock={({ block, children }) => (
          <div key={block.id} data-testid="editor-block">
            {children}
          </div>
        )}
      />,
    );

    expect(screen.getAllByTestId('editor-block')).toHaveLength(2);
  });

  it('sanitizes unsafe rendered hrefs to inert fallbacks', () => {
    const draft = buildDraft();
    draft.navigation.primary[1] = {
      label: 'Bad nav',
      href: 'javascript:alert(1)',
    };
    draft.pages[0]!.blocks[0]!.props.primaryCta = {
      label: 'Unsafe CTA',
      href: 'data:text/html;base64,PHNjcmlwdD4=',
    };

    render(<SiteDraftRenderer site={draft} />);

    expect(
      screen.getByRole('link', { name: 'Bad nav' }).getAttribute('href'),
    ).toBe('#');
    expect(
      screen.getByRole('link', { name: 'Unsafe CTA' }).getAttribute('href'),
    ).toBe('#');
  });
});

function buildDraft(): SiteDraft {
  return {
    site: {
      id: 'site-1',
      name: 'Loom & Light',
      slug: 'loom-light',
      status: 'draft',
      seo: {
        description: 'Warm, small-business websites spun up from a prompt.',
      },
    },
    brand: {
      businessName: 'Loom & Light',
      primaryColor: '#8ee2d1',
    },
    theme: {
      version: 'theme.v1',
      tokens: {
        colors: {
          background: '#151215',
          foreground: '#f3ead8',
          surface: '#231c24',
          surfaceMuted: '#302333',
          primary: '#8ee2d1',
          secondary: '#8fc6ff',
          accent: '#ff8cad',
          border: '#5a3e57',
          muted: '#caa778',
          ring: '#f3b547',
        },
        typography: {
          headingFont: 'Iowan Old Style',
          bodyFont: 'Avenir Next',
        },
        layout: {
          sectionPaddingX: '24px',
          sectionPaddingY: '96px',
          contentWidth: '720px',
        },
        shape: {
          radius: '28px',
        },
      },
    },
    navigation: {
      primary: [
        {
          label: 'Start',
          pageId: 'page-home',
        },
        {
          label: 'Contact',
          href: '/contact',
        },
      ],
    },
    pages: [
      {
        id: 'page-home',
        title: 'Home',
        slug: '/',
        blocks: [
          {
            id: 'block-hero',
            type: 'hero',
            version: 'hero.v1',
            props: {
              eyebrow: 'Prompt-built sites',
              headline: 'Publish a proper site before lunch.',
              subheadline: 'Friendly yarn for colder days.',
              image: {
                assetId: 'asset-hero',
                alt: 'Studio shelves in warm afternoon light',
              },
              primaryCta: {
                label: 'Book a visit',
                href: '/contact',
              },
            },
            settings: {},
          },
          {
            id: 'block-hidden',
            type: 'text_section',
            version: 'text_section.v1',
            props: {
              heading: 'Hidden note',
              body: 'This hidden block should never render.',
            },
            settings: {
              hidden: true,
            },
          },
        ],
      },
      {
        id: 'page-contact',
        title: 'Contact',
        slug: '/contact',
        blocks: [
          {
            id: 'block-contact',
            type: 'text_section',
            version: 'text_section.v1',
            props: {
              heading: 'Drop by the shop',
              body: 'Open weekdays and most Saturdays.',
            },
            settings: {},
          },
        ],
      },
    ],
  };
}

function buildExtendedDraft(): SiteDraft {
  return {
    ...buildDraft(),
    pages: [
      {
        id: 'page-home',
        title: 'Home',
        slug: '/',
        blocks: [
          {
            id: 'block-gallery',
            type: 'gallery',
            version: '1.0.0',
            props: {
              heading: 'Selected work',
              intro: 'A tighter visual proof section.',
              layout: 'grid',
              images: [
                {
                  title: 'Signature image',
                  caption: 'A crafted placeholder for the first gallery slot.',
                },
              ],
            },
            settings: {},
          },
          {
            id: 'block-testimonials',
            type: 'testimonials',
            version: '1.0.0',
            props: {
              heading: 'What clients tend to notice',
              items: [
                {
                  quote: 'Calm process, sharp result.',
                  name: 'Mira',
                  role: 'Client',
                },
              ],
            },
            settings: {},
          },
          {
            id: 'block-pricing',
            type: 'pricing_packages',
            version: '1.0.0',
            props: {
              heading: 'Packages and starting points',
              plans: [
                {
                  name: 'Starter',
                  price: 'From $350',
                  description: 'A compact package.',
                  features: [{ text: 'Focused scope' }],
                  cta: {
                    label: 'Ask about fit',
                    href: '/contact',
                  },
                },
              ],
            },
            settings: {},
          },
          {
            id: 'block-faq',
            type: 'faq',
            version: '1.0.0',
            props: {
              heading: 'Questions people ask before they reach out',
              items: [
                {
                  question: 'What should I send first?',
                  answer: 'A short note about timing and scope.',
                },
              ],
            },
            settings: {},
          },
          {
            id: 'block-team',
            type: 'team_profile_cards',
            version: '1.0.0',
            props: {
              heading: 'People behind the work',
              people: [
                {
                  name: 'Founder',
                  role: 'Lead contact',
                  bio: 'Owns the work and the relationship.',
                  links: [
                    {
                      label: 'Contact',
                      href: '/contact',
                    },
                  ],
                },
              ],
            },
            settings: {},
          },
          {
            id: 'block-footer',
            type: 'footer',
            version: '1.0.0',
            props: {
              siteName: 'Loom & Light',
              tagline: 'A short closing line.',
              copyright: 'Copyright 2026 Loom & Light',
              navigationLinks: [
                {
                  label: 'Contact',
                  href: '/contact',
                },
              ],
            },
            settings: {},
          },
        ],
      },
      {
        id: 'page-contact',
        title: 'Contact',
        slug: '/contact',
        blocks: [],
      },
    ],
  };
}

function buildContactFormDraft(): SiteDraft {
  return {
    ...buildDraft(),
    pages: [
      {
        id: 'page-home',
        title: 'Home',
        slug: '/',
        blocks: [
          {
            id: 'block-contact',
            type: 'contact_form',
            version: '1.0.0',
            props: {
              heading: 'Start the conversation',
              intro: 'Share a few details and I will get back to you shortly.',
              submitLabel: 'Send inquiry',
              fields: [
                {
                  name: 'email',
                  label: 'Email',
                  type: 'email',
                  required: true,
                },
                {
                  name: 'message',
                  label: 'Message',
                  type: 'message',
                  required: true,
                },
              ],
              successMessage: 'Thanks. Your message is on its way.',
            },
            settings: {},
          },
        ],
      },
    ],
  };
}
