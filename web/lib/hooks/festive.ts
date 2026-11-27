"use client";

import { useEffect, useState, useSyncExternalStore } from "react";
import { isThemeId, themeById, themeOn, type FestiveTheme } from "@/lib/domain/season";

const EVENT = "kairso-festive-change";

/** Per-browser festive preferences, kept in localStorage and shared across hooks and tabs. */
function readKey(key: string): string | null {
  try {
    return localStorage.getItem(key);
  } catch {
    return null;
  }
}

function writeKey(key: string, value: string) {
  try {
    localStorage.setItem(key, value);
  } catch {
    /* private mode: the choice lasts for this page only */
  }
  window.dispatchEvent(new Event(EVENT));
}

function subscribe(cb: () => void) {
  window.addEventListener(EVENT, cb);
  window.addEventListener("storage", cb);
  return () => {
    window.removeEventListener(EVENT, cb);
    window.removeEventListener("storage", cb);
  };
}

/** A preference stored as a string; the server render sees the default (parse(null)). */
function usePref<T>(key: string, parse: (raw: string | null) => T, serialize: (v: T) => string = String): [T, (v: T) => void] {
  const raw = useSyncExternalStore(subscribe, () => readKey(key), () => null);
  return [parse(raw), (v: T) => writeKey(key, serialize(v))];
}

/** Whether the viewer wants festive decorations (on by default). */
export function useFestiveEnabled(): [boolean, (on: boolean) => void] {
  return usePref("kairso-festive", (r) => r !== "off", (on) => (on ? "on" : "off"));
}

export type FestiveDensity = "low" | "medium" | "high" | "max";
export const DENSITIES: FestiveDensity[] = ["low", "medium", "high", "max"];
/** How many particles each density spawns, relative to the default. */
export const DENSITY_FACTOR: Record<FestiveDensity, number> = { low: 0.5, medium: 1, high: 1.8, max: 2.8 };

/** How many drifting particles to spawn. */
export function useFestiveDensity(): [FestiveDensity, (d: FestiveDensity) => void] {
  return usePref<FestiveDensity>("kairso-festive-density", (r) => (DENSITIES.includes(r as FestiveDensity) ? (r as FestiveDensity) : "medium"));
}

/** Background music volume, 0..100. */
export function useFestiveVolume(): [number, (v: number) => void] {
  return usePref("kairso-festive-volume", (r) => {
    const n = Number(r);
    return r !== null && Number.isFinite(n) ? Math.min(100, Math.max(0, n)) : 60;
  });
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
