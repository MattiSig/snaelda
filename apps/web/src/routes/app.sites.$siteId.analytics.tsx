import { Link, createFileRoute } from "@tanstack/react-router";
import { useEffect, useMemo, useState } from "react";
import { ArrowLeft, BarChart3 } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  APIError,
  getSiteAnalytics,
  type AnalyticsWindow,
  type SiteAnalytics,
} from "@/lib/api";
import { actions, emptyState, paddedPanel, text } from "@/lib/styles";
import { cn } from "@/lib/utils";

export const Route = createFileRoute("/app/sites/$siteId/analytics")({
  validateSearch: (search: Record<string, unknown>) => ({
    window:
      search.window === "7d" || search.window === "30d" || search.window === "all"
        ? (search.window as AnalyticsWindow)
        : ("7d" as AnalyticsWindow),
  }),
  component: SiteAnalyticsView,
});

const WINDOWS: { value: AnalyticsWindow; label: string }[] = [
  { value: "7d", label: "Last 7 days" },
  { value: "30d", label: "Last 30 days" },
  { value: "all", label: "All time" },
];

function SiteAnalyticsView() {
  const { siteId } = Route.useParams();
  const search = Route.useSearch();
  const navigate = Route.useNavigate();
  const window = search.window;

  const [analytics, setAnalytics] = useState<SiteAnalytics | null>(null);
  const [status, setStatus] = useState<"loading" | "ready" | "error">("loading");
  const [errorMessage, setErrorMessage] = useState("");

  useEffect(() => {
    let isMounted = true;

    getSiteAnalytics(siteId, window)
      .then((response) => {
        if (!isMounted) return;
        setAnalytics(response.analytics);
        setStatus("ready");
      })
      .catch((error) => {
        if (!isMounted) return;
        setErrorMessage(
          error instanceof APIError
            ? error.message
            : "Could not load analytics for this site.",
        );
        setStatus("error");
      });

    return () => {
      isMounted = false;
    };
  }, [siteId, window]);

  function handleWindowChange(next: AnalyticsWindow) {
    navigate({ search: { window: next }, replace: true });
  }

  return (
    <div className="grid gap-5">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-3">
          <Link
            to="/app/sites/$siteId"
            params={{ siteId }}
            search={{ panel: undefined }}
            className={actions.inlineLink}
          >
            <ArrowLeft className="size-4" />
            Back to builder
          </Link>
          <div>
            <p className={text.eyebrow}>Site analytics</p>
            <h1 className={cn(text.h2, "mt-1")}>Traffic</h1>
          </div>
        </div>
        <div role="tablist" aria-label="Analytics window" className={actions.row}>
          {WINDOWS.map((option) => (
            <Button
              key={option.value}
              type="button"
              variant={option.value === window ? "default" : "outline"}
              size="sm"
              onClick={() => handleWindowChange(option.value)}
              aria-pressed={option.value === window}
            >
              {option.label}
            </Button>
          ))}
        </div>
      </header>

      {status === "loading" ? (
        <section className={paddedPanel} aria-live="polite">
          <p className={text.p}>Loading site traffic...</p>
        </section>
      ) : status === "error" ? (
        <section className={paddedPanel}>
          <p className={text.error}>{errorMessage}</p>
        </section>
      ) : analytics ? (
        <AnalyticsContent analytics={analytics} />
      ) : null}
    </div>
  );
}

function AnalyticsContent({ analytics }: { analytics: SiteAnalytics }) {
  const hasViews = analytics.totalViews > 0;
  const dailyMax = useMemo(
    () => analytics.dailyViews.reduce((acc, day) => Math.max(acc, day.viewCount), 0),
    [analytics.dailyViews],
  );

  return (
    <div className="grid gap-5">
      <section
        className={cn(
          paddedPanel,
          "flex flex-wrap items-baseline justify-between gap-3",
        )}
      >
        <div>
          <p className={text.eyebrow}>Total views</p>
          <p className="mt-1 text-[2.6rem] font-black leading-none text-[var(--paper)]">
            {analytics.totalViews.toLocaleString()}
          </p>
        </div>
        <p className={cn(text.muted, "text-sm")}>{formatRange(analytics)}</p>
      </section>

      <section className={paddedPanel}>
        <header className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <p className={text.eyebrow}>Daily views</p>
            <h2 className={cn(text.h3, "mt-1")}>Trend</h2>
          </div>
          <p className={cn(text.muted, "text-sm")}>
            {analytics.dailyViews.length} day{analytics.dailyViews.length === 1 ? "" : "s"}
          </p>
        </header>

        {analytics.dailyViews.length === 0 ? (
          <p className={cn(emptyState, "mt-4")}>
            No views recorded for this window yet.
          </p>
        ) : (
          <div className="mt-4 grid gap-2">
            <div className="flex items-end gap-[3px] h-32 rounded-[12px] bg-[var(--surface-2)] px-3 py-3">
              {analytics.dailyViews.map((day) => {
                const ratio = dailyMax === 0 ? 0 : day.viewCount / dailyMax;
                const height = Math.max(2, Math.round(ratio * 100));
                return (
                  <div
                    key={day.date}
                    className="flex-1 self-end rounded-t-[3px] bg-[var(--thread-teal)] transition-[height]"
                    style={{ height: `${height}%` }}
                    title={`${day.date}: ${day.viewCount}`}
                    aria-label={`${day.date}: ${day.viewCount} views`}
                  />
                );
              })}
            </div>
            <div className="flex justify-between text-xs text-[var(--paper-muted)]">
              <span>{analytics.dailyViews[0]?.date}</span>
              <span>
                {analytics.dailyViews[analytics.dailyViews.length - 1]?.date}
              </span>
            </div>
          </div>
        )}
      </section>

      <section className={paddedPanel}>
        <header className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <p className={text.eyebrow}>Views by page</p>
            <h2 className={cn(text.h3, "mt-1")}>Pages</h2>
          </div>
        </header>

        {!hasViews ? (
          <p className={cn(emptyState, "mt-4 inline-flex items-center gap-2")}>
            <BarChart3 className="size-4 opacity-70" />
            No page views yet — publish and visit your site to start tracking.
          </p>
        ) : (
          <ul className="mt-4 grid gap-2">
            {analytics.pages.map((page) => {
              const ratio = analytics.totalViews === 0
                ? 0
                : page.viewCount / analytics.totalViews;
              return (
                <li
                  key={page.pageId}
                  className="grid gap-1 rounded-[12px] bg-[var(--surface-2)] px-3.5 py-3"
                >
                  <div className="flex flex-wrap items-baseline justify-between gap-2">
                    <span className="text-sm font-bold text-[var(--paper)]">
                      {page.title || page.slug || "Untitled page"}
                    </span>
                    <span className="text-sm font-black text-[var(--paper)]">
                      {page.viewCount.toLocaleString()}
                    </span>
                  </div>
                  {page.slug ? (
                    <span className="text-xs text-[var(--paper-muted)]">
                      {page.slug}
                    </span>
                  ) : null}
                  <div className="h-1.5 overflow-hidden rounded-full bg-[var(--surface-1)]">
                    <div
                      className="h-full rounded-full bg-[var(--thread-teal)]"
                      style={{ width: `${Math.max(2, Math.round(ratio * 100))}%` }}
                    />
                  </div>
                </li>
              );
            })}
          </ul>
        )}
      </section>
    </div>
  );
}

function formatRange(analytics: SiteAnalytics): string {
  if (!analytics.rangeStart || !analytics.rangeEnd) {
    return "All time";
  }
  return `${formatDate(analytics.rangeStart)} – ${formatDate(analytics.rangeEnd)}`;
}

function formatDate(iso: string): string {
  try {
    return new Date(iso).toLocaleDateString(undefined, {
      month: "short",
      day: "numeric",
      year: "numeric",
    });
  } catch {
    return iso.slice(0, 10);
  }
}
