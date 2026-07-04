/// <reference types="vite/client" />
import {
  HeadContent,
  Link,
  Scripts,
  createRootRoute,
  useMatches,
  useRouterState,
} from "@tanstack/react-router";
import type { ReactNode } from "react";
import { useEffect } from "react";
import { Button } from "@/components/ui/button";
import { DefaultCatchBoundary } from "~/components/DefaultCatchBoundary";
import { NotFound } from "~/components/NotFound";
import type { HostedPublicSiteContext } from "~/lib/public-site";
import { useLocale } from "~/lib/locale";
import { topbar } from "~/lib/styles";
import "~/styles/app.css";

export const Route = createRootRoute({
  loader: () => ({
    // Runtime server env (not import.meta.env): the Docker image is built
    // without Railway variables, so a VITE_ var would inline as undefined.
    gaMeasurementId:
      typeof process !== "undefined"
        ? (process.env.GA_MEASUREMENT_ID ?? "")
        : "",
  }),
  head: () => ({
    meta: [
      { charSet: "utf-8" },
      {
        name: "viewport",
        content: "width=device-width, initial-scale=1, viewport-fit=cover",
      },
      { title: "Snaelda" },
      {
        name: "description",
        content: "Structured website drafts, editing, preview, and publishing.",
      },
    ],
    links: [
      { rel: "icon", href: "/logo.png", type: "image/png" },
      { rel: "preconnect", href: "https://fonts.googleapis.com" },
      {
        rel: "preconnect",
        href: "https://fonts.gstatic.com",
        crossOrigin: "anonymous",
      },
      {
        rel: "stylesheet",
        href: "https://fonts.googleapis.com/css2?family=Be+Vietnam+Pro:wght@400;600;700&family=Literata:opsz,wght@7..72,400..700&display=swap",
      },
    ],
  }),
  errorComponent: DefaultCatchBoundary,
  notFoundComponent: () => <NotFound />,
  shellComponent: RootDocument,
});

function RootDocument({ children }: { children: ReactNode }) {
  const matches = useMatches();
  const visitorLocale = useLocale();
  const pathname = useRouterState({
    select: (state) => state.location.pathname,
  });
  const forceDark =
    pathname === "/" ||
    pathname === "/login" ||
    pathname.startsWith("/app") ||
    pathname.startsWith("/restore");

  useEffect(() => {
    const storedMode = window.localStorage.getItem("snaelda-color-mode");
    const nextMode = storedMode === "light" ? "light" : "dark";
    document.documentElement.classList.toggle(
      "dark",
      forceDark || nextMode === "dark",
    );
  }, [forceDark]);

  const hostedPublic =
    matches
      .map((match) => {
        const loaderData = match.loaderData as
          | { hostedPublic?: HostedPublicSiteContext }
          | undefined;
        return loaderData?.hostedPublic ?? null;
      })
      .find((value) => value?.isHostedPublic) ?? null;

  const rootLoaderData = matches[0]?.loaderData as
    | { gaMeasurementId?: string }
    | undefined;
  const rawGaId = rootLoaderData?.gaMeasurementId ?? "";
  // Only the Snaelda marketing/app surface is measured — customers' hosted
  // sites must not carry our analytics tag. The ID lands in an inline script,
  // so accept only the strict GA4 shape.
  const gaMeasurementId =
    !hostedPublic?.isHostedPublic && /^G-[A-Z0-9]+$/.test(rawGaId)
      ? rawGaId
      : "";
  // Hosted public sites render their content locale; the published-site
  // localization work replaces this `en` fallback with the site's default
  // locale. The app + marketing surface follows the visitor's resolved locale.
  const htmlLang = hostedPublic?.isHostedPublic ? "en" : visitorLocale;
  const showChrome =
    !hostedPublic?.isHostedPublic &&
    pathname !== "/" &&
    pathname !== "/terms" &&
    pathname !== "/privacy" &&
    !pathname.startsWith("/app") &&
    !pathname.startsWith("/public/") &&
    !pathname.startsWith("/preview/");

  function toggleColorMode() {
    if (forceDark) {
      return;
    }

    const nextMode = document.documentElement.classList.contains("dark")
      ? "light"
      : "dark";
    document.documentElement.classList.toggle("dark", nextMode === "dark");
    window.localStorage.setItem("snaelda-color-mode", nextMode);
  }

  return (
    <html lang={htmlLang} className="dark" suppressHydrationWarning>
      <head>
        <HeadContent />
        {gaMeasurementId ? (
          <>
            <script
              async
              src={`https://www.googletagmanager.com/gtag/js?id=${gaMeasurementId}`}
            />
            <script
              dangerouslySetInnerHTML={{
                __html: [
                  "window.dataLayer = window.dataLayer || [];",
                  "function gtag(){dataLayer.push(arguments);}",
                  // Consent Mode v2: ads stay denied; analytics is granted
                  // by default so reports actually populate. Once a consent
                  // banner ships, flip analytics_storage back to denied and
                  // let the banner grant it.
                  "gtag('consent', 'default', {",
                  "  ad_storage: 'denied',",
                  "  ad_user_data: 'denied',",
                  "  ad_personalization: 'denied',",
                  "  analytics_storage: 'granted',",
                  "});",
                  "gtag('js', new Date());",
                  `gtag('config', '${gaMeasurementId}');`,
                ].join("\n"),
              }}
            />
          </>
        ) : null}
      </head>
      <body>
        {showChrome ? (
          <header className={topbar.shell}>
            <Link
              to="/"
              className={topbar.brand}
              activeOptions={{ exact: true }}
            >
              <img
                src="/logo.png"
                alt=""
                className="size-8 rounded-full object-contain"
              />
              Snaelda
            </Link>
            <div className={topbar.controls}>
              <nav aria-label="Primary navigation" className={topbar.nav}>
                <Link
                  to="/login"
                  className={topbar.link}
                >
                  Sign in
                </Link>
                <Link to="/app" className={topbar.link}>
                  Builder
                </Link>
                <Link
                  to="/preview/$token"
                  params={{ token: "local" }}
                  className={topbar.link}
                >
                  Preview
                </Link>
                <Link
                  to="/public/$siteSlug"
                  params={{ siteSlug: "local" }}
                  className={topbar.link}
                >
                  Public
                </Link>
              </nav>
              {forceDark ? null : (
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  className="max-sm:min-h-11"
                  onClick={toggleColorMode}
                >
                  Toggle color mode
                </Button>
              )}
            </div>
          </header>
        ) : null}
        {children}
        <Scripts />
      </body>
    </html>
  );
}
