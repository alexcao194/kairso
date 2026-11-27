"use client";

import { useEffect, useSyncExternalStore } from "react";
import type { FestiveTheme } from "@/lib/domain/season";
import { FestivePlayer } from "./player";
import { PLAYLIST, type Score } from "./scores";

/**
 * Festive background music: off by default, switched on by the viewer and remembered per browser.
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
    return localStorage.getItem(KEY) === "on";
  } catch {
    return false;
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

async function start() {
  if (!currentTheme) return;
  player ??= new FestivePlayer();
  player.onTrack = (track) => set({ track });
  try {
    await player.play(PLAYLIST[currentTheme.id]);
    set({ on: true });
  } catch {
    set({ on: false, track: null });
  }
}

function stop() {
  player?.stop();
  set({ on: false, track: null });
}

/** Switch the music on or off (call from a click so the browser allows audio). */
export function toggleFestiveMusic() {
  if (state.on) {
    writePref(false);
    stop();
  } else {
    writePref(true);
    void start();
  }
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
    const resume = () => {
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
