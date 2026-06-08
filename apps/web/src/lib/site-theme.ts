import type { CSSProperties } from "react";
import type {
  SiteDraft,
  ThemeEditorCatalog,
  ThemeSelection,
} from "@/lib/api";

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
  const typeScale = resolveTypeScale(asString(typography.scale));

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
    ...typeScale,
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

export function buildSiteThemeFromSelection(
  currentTheme: ThemeConfig,
  selection: ThemeSelection,
  options: ThemeEditorCatalog,
): ThemeConfig {
  const palette = options.palettes.find(
    (option) => option.id === selection.palette,
  );

  return {
    version: currentTheme.version,
    tokens: {
      colors: {
        ...currentTheme.tokens.colors,
        ...(palette?.previewColors ?? {}),
      },
      typography: {
        ...currentTheme.tokens.typography,
        ...fontPresetTokens(selection.fontPreset),
        scale: selection.typeScale,
      },
      layout: {
        ...currentTheme.tokens.layout,
        sectionPaddingY: sectionSpacingValue(selection.sectionSpacing),
        contentWidth: contentWidthValue(selection.contentWidth),
      },
      shape: {
        ...currentTheme.tokens.shape,
        radius: radiusValue(selection.radius),
        buttonStyle: selection.buttonStyle,
        imageStyle: selection.imageStyle,
      },
    },
  };
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
    case "Literata":
      return '"Literata", "Iowan Old Style", "Palatino Linotype", Georgia, serif';
    case "Be Vietnam Pro":
      return '"Be Vietnam Pro", "Avenir Next", "Segoe UI", sans-serif';
    case "Iowan Old Style":
      return '"Iowan Old Style", "Palatino Linotype", "Book Antiqua", Georgia, serif';
    case "Avenir Next":
      return '"Avenir Next", "Segoe UI", "Helvetica Neue", sans-serif';
    case "Helvetica Neue":
      return '"Helvetica Neue", Helvetica, Arial, sans-serif';
    case "Trebuchet MS":
      return '"Trebuchet MS", "Segoe UI", Arial, sans-serif';
    case "Georgia":
      return 'Georgia, "Times New Roman", serif';
    default:
      return value || '"Avenir Next", "Segoe UI", "Helvetica Neue", sans-serif';
  }
}

function resolveTypeScale(value: string): Record<string, string> {
  switch (value) {
    case "compact":
      return {
        "--size-sectionHeading": "clamp(1.5rem,2.4vw,2.1rem)",
        "--size-heroHeading": "clamp(2.25rem,5vw,4.25rem)",
        "--size-fullPageHeading": "clamp(2.5rem,6vw,4.8rem)",
      };
    case "expressive":
      return {
        "--size-sectionHeading": "clamp(1.9rem,3.4vw,2.9rem)",
        "--size-heroHeading": "clamp(3.2rem,7.5vw,6.6rem)",
        "--size-fullPageHeading": "clamp(3.4rem,8vw,7rem)",
      };
    default:
      return {
        "--size-sectionHeading": "clamp(1.65rem,2.8vw,2.4rem)",
        "--size-heroHeading": "clamp(2.6rem,6.2vw,5.4rem)",
        "--size-fullPageHeading": "clamp(2.8rem,7vw,6rem)",
      };
  }
}

function fontPresetTokens(value: string): Record<string, string | number> {
  switch (value) {
    case "editorial":
      return {
        heading: "Literata",
        body: "Literata",
        headingFont: "Literata",
        bodyFont: "Literata",
        headingWeight: 700,
        bodyWeight: 400,
      };
    case "studio-sans":
      return {
        heading: "Be Vietnam Pro",
        body: "Be Vietnam Pro",
        headingFont: "Be Vietnam Pro",
        bodyFont: "Be Vietnam Pro",
        headingWeight: 700,
        bodyWeight: 400,
      };
    case "modern-grotesk":
      return {
        heading: "Helvetica Neue",
        body: "Helvetica Neue",
        headingFont: "Helvetica Neue",
        bodyFont: "Helvetica Neue",
        headingWeight: 700,
        bodyWeight: 400,
      };
    case "humanist":
      return {
        heading: "Trebuchet MS",
        body: "Trebuchet MS",
        headingFont: "Trebuchet MS",
        bodyFont: "Trebuchet MS",
        headingWeight: 700,
        bodyWeight: 400,
      };
    case "heritage-serif":
      return {
        heading: "Georgia",
        body: "Georgia",
        headingFont: "Georgia",
        bodyFont: "Georgia",
        headingWeight: 700,
        bodyWeight: 400,
      };
    default:
      return {
        heading: "Literata",
        body: "Be Vietnam Pro",
        headingFont: "Literata",
        bodyFont: "Be Vietnam Pro",
        headingWeight: 600,
        bodyWeight: 400,
      };
  }
}

function sectionSpacingValue(value: string) {
  switch (value) {
    case "snug":
      return "88px";
    case "airy":
      return "120px";
    default:
      return "96px";
  }
}

function contentWidthValue(value: string) {
  switch (value) {
    case "focused":
      return "640px";
    case "wide":
      return "860px";
    default:
      return "720px";
  }
}

function radiusValue(value: string) {
  switch (value) {
    case "sharp":
      return "0px";
    case "crisp":
      return "8px";
    case "tailored":
      return "16px";
    case "pillowy":
      return "32px";
    default:
      return "24px";
  }
}

function innerRadius(radius: string) {
  const pixelRadius = /^(\d+(?:\.\d+)?)px$/.exec(radius);
  if (pixelRadius) {
    return `${Math.max(0, Number(pixelRadius[1]) - 6)}px`;
  }
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
