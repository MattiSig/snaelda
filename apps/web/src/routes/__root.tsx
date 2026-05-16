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
import { topbar } from "~/lib/styles";
import appCss from "~/styles/app.css?url";

export const Route = createRootRoute({
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
      { rel: "stylesheet", href: appCss },
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
  const showChrome =
    !hostedPublic?.isHostedPublic &&
    pathname !== "/" &&
    !pathname.startsWith("/app") &&
    !pathname.startsWith("/public/");

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
    <html lang="en" className="dark" suppressHydrationWarning>
      <head>
        <HeadContent />
      </head>
      <body>
        {showChrome ? (
          <header className={topbar.shell}>
            <Link
              to="/"
              search={{ restore: '' }}
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
