import type { CSSProperties } from "react";

/** The nine decorative hues, plus "accent": the workspace brand colour. */
export type Hue = "accent" | "blue" | "violet" | "pink" | "rose" | "orange" | "amber" | "green" | "teal" | "cyan";

export const hueVars = (h: Hue): CSSProperties =>
  h === "accent"
    ? ({ "--h": "var(--accent-ink)", "--hs": "var(--accent-soft)", "--hl": "var(--accent-border)", "--hv": "var(--accent)" } as CSSProperties)
    : ({ "--h": `var(--${h})`, "--hs": `var(--${h}-soft)`, "--hl": `var(--${h}-line)`, "--hv": `var(--${h}-vivid)` }) as CSSProperties;

export const NAV_HUE = {
  home: "accent",
  inbox: "rose",
  myTasks: "blue",
  threads: "cyan",
  calendar: "orange",
  initiatives: "amber",
  analytics: "pink",
  files: "blue",
  quotes: "teal",
  settings: "green",
} as const satisfies Record<string, Hue>;
