"use client";

import { useEffect, useState, useSyncExternalStore } from "react";
import { isThemeId, themeById, themeOn, type FestiveTheme } from "@/lib/domain/season";

const KEY = "kairso-festive";
const EVENT = "kairso-festive-change";

function readEnabled(): boolean {
  try {
    return localStorage.getItem(KEY) !== "off";
  } catch {
    return true;
  }
}

function subscribe(cb: () => void) {
  window.addEventListener(EVENT, cb);
  window.addEventListener("storage", cb);
  return () => {
    window.removeEventListener(EVENT, cb);
    window.removeEventListener("storage", cb);
  };
}

/** Whether the viewer wants festive decorations (per browser; on by default). */
export function useFestiveEnabled(): [boolean, (on: boolean) => void] {
  const on = useSyncExternalStore(subscribe, readEnabled, () => true);
  const set = (v: boolean) => {
    try {
      localStorage.setItem(KEY, v ? "on" : "off");
    } catch {
      /* private mode: the choice lasts for this page only */
    }
    window.dispatchEvent(new Event(EVENT));
  };
  return [on, set];
}

/**
 * Today's festive theme, or null when decorations are off. `?season=<id>` previews any theme
 * (christmas, tet, autumn…). Computed after mount so server and client render the same.
 */
export function useFestiveTheme(): FestiveTheme | null {
  const [enabled] = useFestiveEnabled();
  const [theme, setTheme] = useState<FestiveTheme | null>(null);
  useEffect(() => {
    const preview = new URLSearchParams(window.location.search).get("season");
    const pick = () => setTheme(isThemeId(preview) ? themeById(preview) : themeOn(new Date()));
    pick();
    // Cross midnight without a reload.
    const t = window.setInterval(pick, 30 * 60_000);
    return () => window.clearInterval(t);
  }, []);
  return enabled ? theme : null;
}

function prefersReducedMotion(): boolean {
  return typeof window !== "undefined" && window.matchMedia("(prefers-reduced-motion: reduce)").matches;
}

/** True when the OS asks for reduced motion: show static ornaments only, no drifting particles. */
export function useReducedMotion(): boolean {
  return useSyncExternalStore(
    (cb) => {
      const mq = window.matchMedia("(prefers-reduced-motion: reduce)");
      mq.addEventListener("change", cb);
      return () => mq.removeEventListener("change", cb);
    },
    prefersReducedMotion,
    () => false,
  );
}
