import type { CSSProperties } from "react";
import type { SiteDraft } from "@/lib/api";

type ThemeConfig = SiteDraft["theme"];

export function buildSiteThemeStyle(theme: ThemeConfig): CSSProperties {
  const colors = theme.tokens.colors ?? {};
  const typography = theme.tokens.typography ?? {};
  const layout = theme.tokens.layout ?? {};
  const shape = theme.tokens.shape ?? {};
  const radius = asString(shape.radius) || "28px";
  const sectionPaddingX = asString(layout.sectionPaddingX) || "24px";
  const sectionPaddingY =
    asString(layout.sectionPaddingY) ||
    asString(layout.sectionSpacing) ||
    "96px";
  const contentWidth = asString(layout.contentWidth) || "720px";
  const buttonStyle = asString(shape.buttonStyle) || "ribbon-fill";
  const imageStyle = asString(shape.imageStyle) || "woven-tint";
  const buttonVars = resolveButtonVars(buttonStyle, colors);
  const imageVars = resolveImageVars(imageStyle, colors);
  const headingFont = resolveFontStack(asString(typography.headingFont));
  const bodyFont = resolveFontStack(asString(typography.bodyFont));
  const headingWeight = asNumberString(typography.headingWeight, "700");
  const bodyWeight = asNumberString(typography.bodyWeight, "400");

  return {
    "--color-background": colors.background ?? "#191119",
    "--color-text": colors.foreground ?? colors.text ?? "#f3ead8",
    "--color-surface": colors.surface ?? "#241a24",
    "--color-surface-muted": colors.surfaceMuted ?? "#302333",
    "--color-primary": colors.primary ?? "#86d8cf",
    "--color-secondary": colors.secondary ?? "#89b9f0",
    "--color-accent": colors.accent ?? "#ff8a9d",
    "--color-border": colors.border ?? "#5a3e57",
    "--color-muted": colors.muted ?? "#caa778",
    "--color-ring": colors.ring ?? "#f2bd63",
    "--font-heading": headingFont,
    "--font-body": bodyFont,
    "--font-headingWeight": headingWeight,
    "--font-bodyWeight": bodyWeight,
    "--size-contentWidth": contentWidth,
    "--radius-panel": radius,
    "--radius-card": radius,
    "--radius-button": radius,
    "--radius-inner": innerRadius(radius),
    "--space-sectionPaddingX": sectionPaddingX,
    "--space-sectionPaddingY": sectionPaddingY,
    "--color-glowPrimary": withAlpha(colors.primary ?? "#86d8cf", "1f"),
    "--color-glowAccent": withAlpha(colors.accent ?? "#ff8a9d", "1f"),
    ...buttonVars,
    ...imageVars,
  } as CSSProperties;
}

function asString(value: unknown) {
  return typeof value === "string" ? value : "";
}

function asNumberString(value: unknown, fallback: string) {
  if (typeof value === "number" && Number.isFinite(value)) {
    return String(value);
  }
  if (typeof value === "string" && value.trim() !== "") {
    return value.trim();
  }
  return fallback;
}

function resolveFontStack(value: string) {
  switch (value) {
    case "Iowan Old Style":
      return '"Iowan Old Style", "Palatino Linotype", "Book Antiqua", Georgia, serif';
    case "Avenir Next":
      return '"Avenir Next", "Segoe UI", "Helvetica Neue", sans-serif';
    default:
      return value || '"Avenir Next", "Segoe UI", "Helvetica Neue", sans-serif';
  }
}

function innerRadius(radius: string) {
  return `calc(${radius} - 6px)`;
}

function withAlpha(color: string, alphaHex: string) {
  const normalized = color.trim();
  if (/^#[0-9a-fA-F]{6}$/.test(normalized)) {
    return `${normalized}${alphaHex}`;
  }
  if (/^#[0-9a-fA-F]{3}$/.test(normalized)) {
    const expanded = normalized
      .slice(1)
      .split("")
      .map((part) => part + part)
      .join("");
    return `#${expanded}${alphaHex}`;
  }
  return color;
}

function resolveButtonVars(style: string, colors: Record<string, string>) {
  const background = colors.background ?? "#191119";
  const foreground = colors.foreground ?? colors.text ?? "#f3ead8";
  const primary = colors.primary ?? "#86d8cf";
  const surface = colors.surface ?? "#241a24";
  const surfaceMuted = colors.surfaceMuted ?? "#302333";
  const border = colors.border ?? "#5a3e57";

  switch (style) {
    case "thread-outline":
      return {
        "--color-buttonBackground": surface,
        "--color-buttonForeground": primary,
        "--color-buttonBorder": primary,
        "--shadow-button": "none",
        "--color-buttonGhostBackground": surfaceMuted,
        "--color-buttonGhostForeground": foreground,
        "--color-buttonGhostBorder": border,
      };
    case "ink-solid":
      return {
        "--color-buttonBackground": foreground,
        "--color-buttonForeground": background,
        "--color-buttonBorder": foreground,
        "--shadow-button": `0 12px 24px ${withAlpha(foreground, "1f")}`,
        "--color-buttonGhostBackground": surfaceMuted,
        "--color-buttonGhostForeground": foreground,
        "--color-buttonGhostBorder": foreground,
      };
    default:
      return {
        "--color-buttonBackground": primary,
        "--color-buttonForeground": background,
        "--color-buttonBorder": primary,
        "--shadow-button": `0 12px 24px ${withAlpha(primary, "2b")}`,
        "--color-buttonGhostBackground": surfaceMuted,
        "--color-buttonGhostForeground": foreground,
        "--color-buttonGhostBorder": border,
      };
  }
}

function resolveImageVars(style: string, colors: Record<string, string>) {
  const surface = colors.surface ?? "#241a24";
  const surfaceMuted = colors.surfaceMuted ?? "#302333";
  const primary = colors.primary ?? "#86d8cf";
  const secondary = colors.secondary ?? "#89b9f0";
  const accent = colors.accent ?? "#ff8a9d";
  const border = colors.border ?? "#5a3e57";
  const background = colors.background ?? "#191119";

  switch (style) {
    case "soft-frame":
      return {
        "--color-imageBackground": surface,
        "--color-imageBorder": border,
        "--shadow-image": "none",
        "--image-tallBackground": `linear-gradient(180deg, ${surface} 0%, ${surfaceMuted} 100%)`,
        "--color-imageCaptionBackground": withAlpha(background, "66"),
      };
    case "paper-cut":
      return {
        "--color-imageBackground": `color-mix(in oklch, ${surface} 88%, ${accent})`,
        "--color-imageBorder": accent,
        "--shadow-image": `0 16px 32px ${withAlpha(accent, "18")}`,
        "--image-tallBackground": `linear-gradient(155deg, color-mix(in oklch, ${surface} 84%, ${accent}) 0%, color-mix(in oklch, ${surfaceMuted} 88%, ${secondary}) 58%, color-mix(in oklch, ${surface} 90%, ${primary}) 100%)`,
        "--color-imageCaptionBackground": withAlpha(surface, "b8"),
      };
    default:
      return {
        "--color-imageBackground": `color-mix(in oklch, ${surface} 88%, ${secondary})`,
        "--color-imageBorder": border,
        "--shadow-image": `0 16px 32px ${withAlpha(primary, "18")}`,
        "--image-tallBackground": `linear-gradient(160deg, color-mix(in oklch, ${surface} 84%, ${primary}) 0%, ${surfaceMuted} 55%, color-mix(in oklch, ${surface} 90%, ${accent}) 100%)`,
        "--color-imageCaptionBackground": withAlpha(surface, "b8"),
      };
  }
}
