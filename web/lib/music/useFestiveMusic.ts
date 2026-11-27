"use client";

import { useEffect, useSyncExternalStore } from "react";
import type { FestiveTheme } from "@/lib/domain/season";
import { FestivePlayer } from "./player";
import { PLAYLIST, type Score } from "./scores";

/**
 * Festive background music: on by default, can be switched off by the viewer (remembered per browser).
 * Browsers only start audio after a user gesture, so a remembered "on" resumes at the first
 * click or key press. One player for the whole app.
 */

const KEY = "kairso-festive-music";

let player: FestivePlayer | null = null;
let state: { on: boolean; track: Score | null } = { on: false, track: null };
const listeners = new Set<() => void>();
const emit = () => listeners.forEach((l) => l());
const set = (next: Partial<typeof state>) => {
  state = { ...state, ...next };
  emit();
};

function readPref(): boolean {
  try {
    return localStorage.getItem(KEY) !== "off";
  } catch {
    return true;
  }
}
function writePref(on: boolean) {
  try {
    localStorage.setItem(KEY, on ? "on" : "off");
  } catch {
    /* private mode */
  }
}

let currentTheme: FestiveTheme | null = null;

/** The saved volume (0..100, default 60) as a master gain; 100 % is still background-quiet. */
function savedGain(): number {
  let v = 60;
  try {
    const n = Number(localStorage.getItem("kairso-festive-volume"));
    if (localStorage.getItem("kairso-festive-volume") !== null && Number.isFinite(n)) v = Math.min(100, Math.max(0, n));
  } catch {
    /* default */
  }
  return (v / 100) * 0.25;
}

/** Apply a new volume (0..100) to the music that is playing. */
export function setFestiveVolume(v: number) {
  player?.setVolume((Math.min(100, Math.max(0, v)) / 100) * 0.25);
}

async function start() {
  if (!currentTheme) return;
  player ??= new FestivePlayer();
  player.onTrack = (track) => set({ track });
  try {
    await player.play(PLAYLIST[currentTheme.id], savedGain());
    set({ on: true });
  } catch {
    set({ on: false, track: null });
  }
}

function stop() {
  player?.stop();
  set({ on: false, track: null });
}

/** Turn the music on or off (call from a click so the browser allows audio). */
export function setFestiveMusic(on: boolean) {
  writePref(on);
  if (on) void start();
  else stop();
}

export function toggleFestiveMusic() {
  setFestiveMusic(!state.on);
}

export function useFestiveMusicState() {
  return useSyncExternalStore(
    (cb) => {
      listeners.add(cb);
      return () => listeners.delete(cb);
    },
    () => state,
    () => state,
  );
}

/** Mount once: follows the theme, stops when decorations are off, resumes a remembered "on". */
export function useFestiveMusicController(theme: FestiveTheme | null) {
  const id = theme?.id ?? null;
  useEffect(() => {
    currentTheme = theme;
    if (!theme) {
      if (state.on) stop();
      return;
    }
    if (state.on) {
      void start(); // theme changed while playing → its own playlist
      return;
    }
    if (!readPref()) return;
    const resume = (e: Event) => {
      // A click on a music control decides by itself (it would otherwise start and stop at once).
      if (e.target instanceof Element && e.target.closest("[data-festive-music]")) return;
      cleanup();
      if (!state.on && readPref()) void start();
    };
    const cleanup = () => {
      window.removeEventListener("pointerdown", resume);
      window.removeEventListener("keydown", resume);
    };
    window.addEventListener("pointerdown", resume);
    window.addEventListener("keydown", resume);
    return cleanup;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id]);

  useEffect(() => () => player?.stop(0), []);
}
