/**
 * Workspace brand colour → full theme. One #RRGGBB value drives every accent token in the UI
 * (light + dark) and the --doc-* tokens used by the PDF/print pages. Contrast is enforced here so
 * any colour — pale yellow, near-black, neon — still yields readable text (WCAG AA, ≥ 4.5:1).
 */

export const BRAND_HEX = /^#[0-9A-Fa-f]{6}$/;
export const BRAND_PRESETS = ["#4F46E5", "#2563EB", "#0891B2", "#059669", "#65A30D", "#D97706", "#EA580C", "#C8102E", "#DB2777", "#7C3AED", "#475569"];

type Rgb = [number, number, number];
interface Hsl { h: number; s: number; l: number }

const clamp = (v: number, lo = 0, hi = 1) => Math.min(hi, Math.max(lo, v));

function hexToRgb(hex: string): Rgb {
  const n = parseInt(hex.slice(1), 16);
  return [(n >> 16) & 255, (n >> 8) & 255, n & 255];
}

function rgbToHsl([r, g, b]: Rgb): Hsl {
  const R = r / 255, G = g / 255, B = b / 255;
  const max = Math.max(R, G, B), min = Math.min(R, G, B), d = max - min;
  const l = (max + min) / 2;
  if (d === 0) return { h: 0, s: 0, l };
  const s = d / (1 - Math.abs(2 * l - 1));
  let h = max === R ? ((G - B) / d) % 6 : max === G ? (B - R) / d + 2 : (R - G) / d + 4;
  h = (h * 60 + 360) % 360;
  return { h, s, l };
}

function hslToRgb({ h, s, l }: Hsl): Rgb {
  const c = (1 - Math.abs(2 * l - 1)) * s;
  const x = c * (1 - Math.abs(((h / 60) % 2) - 1));
  const m = l - c / 2;
  const [r, g, b] = h < 60 ? [c, x, 0] : h < 120 ? [x, c, 0] : h < 180 ? [0, c, x] : h < 240 ? [0, x, c] : h < 300 ? [x, 0, c] : [c, 0, x];
  return [Math.round((r + m) * 255), Math.round((g + m) * 255), Math.round((b + m) * 255)];
}

const toHex = (rgb: Rgb) => `#${rgb.map((v) => v.toString(16).padStart(2, "0")).join("").toUpperCase()}`;
const hsl = (h: number, s: number, l: number): Hsl => ({ h: (h + 360) % 360, s: clamp(s), l: clamp(l) });
const hex = (c: Hsl) => toHex(hslToRgb(c));

function luminance([r, g, b]: Rgb): number {
  const f = (v: number) => { const c = v / 255; return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4; };
  return 0.2126 * f(r) + 0.7152 * f(g) + 0.0722 * f(b);
}

export function contrast(a: string, b: string): number {
  const la = luminance(hexToRgb(a)), lb = luminance(hexToRgb(b));
  return (Math.max(la, lb) + 0.05) / (Math.min(la, lb) + 0.05);
}

/** Move lightness away from `bg` until `c` reaches `min` contrast against it. */
function ensure(c: Hsl, bg: string, min: number): Hsl {
  const darker = luminance(hexToRgb(bg)) > 0.4;
  let cur = c;
  for (let i = 0; i < 100 && contrast(hex(cur), bg) < min; i++) cur = { ...cur, l: clamp(cur.l + (darker ? -0.01 : 0.01)) };
  return cur;
}

const rgba = (c: string, a: number) => { const [r, g, b] = hexToRgb(c); return `rgba(${r}, ${g}, ${b}, ${a})`; };
const WHITE = "#FFFFFF";
const NEAR_BLACK = "#111827";

/** Pick white or near-black text for a solid background, whichever reads better. */
const onColor = (bg: string) => (contrast(bg, WHITE) >= contrast(bg, NEAR_BLACK) ? WHITE : NEAR_BLACK);

export function normalizeBrand(v?: string | null): string | null {
  return v && BRAND_HEX.test(v) ? v.toUpperCase() : null;
}

export const DEFAULT_BRAND = "#4F46E5";
const DARK_SURFACE = "#19191C";

/** Accent tokens for the app UI (light theme). */
function lightTokens(base: Hsl): Record<string, string> {
  const baseHex = hex(base);
  // Keep the picked colour when white text is readable on it; otherwise prefer dark text on a
  // bright colour, and only darken the colour as a last resort.
  let accent = base;
  let on = WHITE;
  if (contrast(baseHex, WHITE) < 4.5) {
    if (contrast(baseHex, NEAR_BLACK) >= 4.5) on = NEAR_BLACK;
    else accent = ensure(base, WHITE, 4.5);
  }
  // A near-white brand would vanish on white surfaces: deepen it until fills are visible.
  if (contrast(hex(accent), WHITE) < 1.6) {
    accent = ensure(accent, WHITE, 3);
    on = onColor(hex(accent));
  }
  const accentHex = hex(accent);
  const s = Math.min(accent.s, 0.9);
  const hover = hsl(accent.h, accent.s * 0.92, accent.l + (on === WHITE ? -0.07 : -0.06));
  const soft = hsl(accent.h, Math.min(s, 0.75), 0.95);
  const softer = hsl(accent.h, Math.min(s, 0.6), 0.975);
  const ink = ensure(ensure(hsl(accent.h, accent.s * 0.82, Math.min(accent.l, 0.45)), WHITE, 5), hex(soft), 4.6);
  const border = hsl(accent.h, Math.min(s, 0.65), 0.85);
  // Hero banner: the brand colour itself, deepened along the same hue, always with white text.
  const hero = ensure(base, WHITE, 4.6);
  const heroEnd = hsl(hero.h, hero.s * 0.85, hero.l - 0.1);
  return {
    "--accent": accentHex,
    "--accent-hover": hex(hover),
    "--accent-ink": hex(ink),
    "--accent-soft": hex(soft),
    "--accent-softer": hex(softer),
    "--accent-border": hex(border),
    "--accent-on": on,
    "--on-white": hex(ensure(hsl(accent.h, accent.s, Math.min(accent.l, 0.42)), WHITE, 5.5)),
    "--hero-bg": `linear-gradient(135deg, ${hex(hero)} 0%, ${hex(heroEnd)} 100%)`,
    "--sh-accent": `0 2px 8px ${rgba(accentHex, 0.22)}`,
  };
}

/** Accent tokens for the dark theme: the brand colour kept recognisable, lifted only as far as
 *  it needs to stand out on dark surfaces, with tinted soft fills. */
function darkTokens(base: Hsl): Record<string, string> {
  const s = clamp(base.s, 0.35, 0.9);
  // Fills (buttons, active states) need 3:1 against the surface (WCAG 1.4.11); text uses --accent-ink.
  const accent = ensure(hsl(base.h, s, clamp(base.l, 0.45, 0.6)), DARK_SURFACE, 3.2);
  const accentHex = hex(accent);
  const on = onColor(accentHex);
  const hover = hsl(accent.h, s, accent.l + 0.06);
  const soft = hsl(accent.h, s * 0.5, 0.18);
  const ink = ensure(hsl(accent.h, s, 0.76), hex(soft), 5);
  const hero = hsl(base.h, s * 0.75, 0.3);
  const heroEnd = hsl(base.h, s * 0.75, 0.21);
  return {
    "--accent": accentHex,
    "--accent-hover": hex(hover),
    "--accent-ink": hex(ink),
    "--accent-soft": hex(soft),
    "--accent-softer": hex(hsl(accent.h, s * 0.35, 0.14)),
    "--accent-border": hex(hsl(accent.h, s * 0.45, 0.34)),
    "--accent-on": on,
    "--on-white": hex(ensure(hsl(accent.h, accent.s, 0.4), WHITE, 5.5)),
    "--hero-bg": `linear-gradient(135deg, ${hex(hero)} 0%, ${hex(heroEnd)} 100%)`,
    "--sh-accent": `0 2px 10px ${rgba(accentHex, 0.2)}`,
  };
}

/** Tokens for printable documents (white paper, brand-coloured header with white text). */
export function docTokens(color?: string | null): Record<string, string> | undefined {
  const c = normalizeBrand(color);
  if (!c) return undefined;
  const base = rgbToHsl(hexToRgb(c));
  const brand = ensure(base, WHITE, 4.5);
  const s = base.s;
  return {
    "--doc-brand": hex(brand),
    "--doc-brand-2": hex(hsl(brand.h + 6, brand.s, brand.l + 0.07)),
    "--doc-tint": hex(hsl(base.h, clamp(s, 0, 0.9), 0.97)),
    "--doc-tint-line": hex(hsl(base.h, clamp(s * 0.6, 0, 0.7), 0.88)),
    "--doc-row-alt": hex(hsl(base.h, clamp(s * 0.6, 0, 0.7), 0.975)),
    "--doc-line": hex(hsl(base.h, clamp(s * 0.2, 0, 0.2), 0.92)),
  };
}

const decl = (t: Record<string, string>) => Object.entries(t).map(([k, v]) => `${k}:${v}`).join(";");

/** Stylesheet text setting the accent tokens for both themes; without a brand colour, the default one. */
export function brandCss(color?: string | null): string {
  const c = normalizeBrand(color) ?? DEFAULT_BRAND;
  const base = rgbToHsl(hexToRgb(c));
  return `html:root{${decl(lightTokens(base))}}\nhtml:root[data-theme="dark"]{${decl(darkTokens(base))}}`;
}

/** Inline CSS variables to preview a colour (UI accent + document header) in a small box. */
export function previewTokens(color?: string | null): Record<string, string> {
  const c = normalizeBrand(color);
  if (!c) return {};
  return { ...lightTokens(rgbToHsl(hexToRgb(c))), ...(docTokens(c) ?? {}) };
}
